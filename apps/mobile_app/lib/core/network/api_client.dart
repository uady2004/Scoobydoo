import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:logger/logger.dart';

import 'api_endpoints.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Storage keys
// ─────────────────────────────────────────────────────────────────────────────
abstract final class _StorageKeys {
  static const String accessToken = 'tk_access';
  static const String refreshToken = 'tk_refresh';
}

// ─────────────────────────────────────────────────────────────────────────────
// Auth Interceptor
// ─────────────────────────────────────────────────────────────────────────────

/// Attaches the JWT Bearer token to every outgoing request.
/// On 401 it attempts a token refresh once then retries.
class _AuthInterceptor extends Interceptor {
  _AuthInterceptor({
    required Dio dio,
    required FlutterSecureStorage storage,
  })  : _dio = dio,
        _storage = storage;

  final Dio _dio;
  final FlutterSecureStorage _storage;

  // Guards against concurrent refresh races.
  bool _isRefreshing = false;
  final List<_PendingRequest> _pendingQueue = [];

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    // Allow callers to skip auth (e.g. the refresh call itself).
    if (options.extra['skip_auth'] == true) {
      handler.next(options);
      return;
    }
    final token = await _storage.read(key: _StorageKeys.accessToken);
    if (token != null && token.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    if (err.response?.statusCode != 401) {
      handler.next(err);
      return;
    }

    // Avoid retrying the refresh endpoint itself.
    if (err.requestOptions.path.contains(ApiEndpoints.refresh)) {
      handler.next(err);
      return;
    }

    if (_isRefreshing) {
      // Queue request until the ongoing refresh completes.
      final completer = Completer<Response<dynamic>>();
      _pendingQueue.add(_PendingRequest(err.requestOptions, completer));
      try {
        final response = await completer.future;
        handler.resolve(response);
      } catch (e) {
        handler.next(err);
      }
      return;
    }

    _isRefreshing = true;

    try {
      final refreshed = await _refresh();
      if (!refreshed) {
        _rejectPending();
        handler.next(err);
        return;
      }

      // Retry the original request with the new token.
      final retried = await _retry(err.requestOptions);
      _resolvePending(retried);
      handler.resolve(retried);
    } catch (_) {
      _rejectPending();
      handler.next(err);
    } finally {
      _isRefreshing = false;
    }
  }

  // ── Helpers ──────────────────────────────────────────────────────────────

  Future<bool> _refresh() async {
    final refreshToken =
        await _storage.read(key: _StorageKeys.refreshToken);
    if (refreshToken == null) return false;

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        ApiEndpoints.refresh,
        data: {'refresh_token': refreshToken},
        options: Options(
          extra: {'skip_auth': true},
        ),
      );

      final data = response.data;
      if (data == null) return false;

      await _storage.write(
        key: _StorageKeys.accessToken,
        value: data['access_token'] as String?,
      );
      if (data['refresh_token'] != null) {
        await _storage.write(
          key: _StorageKeys.refreshToken,
          value: data['refresh_token'] as String,
        );
      }
      return true;
    } on DioException {
      return false;
    }
  }

  Future<Response<dynamic>> _retry(RequestOptions options) async {
    final token = await _storage.read(key: _StorageKeys.accessToken);
    final opts = Options(
      method: options.method,
      headers: {
        ...options.headers,
        'Authorization': 'Bearer $token',
      },
      extra: {'skip_auth': true},
    );
    return _dio.request<dynamic>(
      options.path,
      data: options.data,
      queryParameters: options.queryParameters,
      options: opts,
    );
  }

  void _resolvePending(Response<dynamic> response) {
    for (final pending in _pendingQueue) {
      pending.completer.complete(response);
    }
    _pendingQueue.clear();
  }

  void _rejectPending() {
    for (final pending in _pendingQueue) {
      pending.completer.completeError(
        DioException(
          requestOptions: pending.options,
          message: 'Token refresh failed',
        ),
      );
    }
    _pendingQueue.clear();
  }
}

class _PendingRequest {
  _PendingRequest(this.options, this.completer);
  final RequestOptions options;
  final Completer<Response<dynamic>> completer;
}

// ─────────────────────────────────────────────────────────────────────────────
// Retry Interceptor
// ─────────────────────────────────────────────────────────────────────────────

/// Retries failed requests up to [maxAttempts] times with exponential backoff
/// (1 s, 2 s, 4 s). Only retries on network-level errors (no response).
class _RetryInterceptor extends Interceptor {
  _RetryInterceptor({required Dio dio}) : _dio = dio;

  final Dio _dio;

  static const int _maxAttempts = 1;
  static const List<int> _backoffSeconds = [1];

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final isNetworkError = err.type == DioExceptionType.connectionError ||
        err.type == DioExceptionType.receiveTimeout ||
        err.type == DioExceptionType.connectionTimeout ||
        err.error is SocketException;

    if (!isNetworkError) {
      handler.next(err);
      return;
    }

    final attempt =
        (err.requestOptions.extra['retry_attempt'] as int?) ?? 0;

    if (attempt >= _maxAttempts - 1) {
      handler.next(err);
      return;
    }

    final delay = Duration(seconds: _backoffSeconds[attempt]);
    await Future<void>.delayed(delay);

    final options = err.requestOptions;
    options.extra['retry_attempt'] = attempt + 1;

    try {
      final response = await _dio.request<dynamic>(
        options.path,
        data: options.data,
        queryParameters: options.queryParameters,
        options: Options(
          method: options.method,
          headers: options.headers,
          extra: options.extra,
        ),
      );
      handler.resolve(response);
    } on DioException catch (retryErr) {
      handler.next(retryErr);
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Logging Interceptor
// ─────────────────────────────────────────────────────────────────────────────

/// Logs request URL, method, and response status in debug builds only.
class _LoggingInterceptor extends Interceptor {
  final Logger _log = Logger(
    printer: PrettyPrinter(
      methodCount: 0,
      errorMethodCount: 5,
      lineLength: 80,
      colors: false,
      printEmojis: false,
      dateTimeFormat: DateTimeFormat.onlyTimeAndSinceStart,
    ),
  );

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    if (kDebugMode) {
      _log.d('[HTTP] --> ${options.method} ${options.uri}');
    }
    handler.next(options);
  }

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) {
    if (kDebugMode) {
      _log.d(
        '[HTTP] <-- ${response.statusCode} '
        '${response.requestOptions.method} '
        '${response.requestOptions.uri}',
      );
    }
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    if (kDebugMode) {
      _log.e(
        '[HTTP] ERROR ${err.response?.statusCode ?? "no-response"} '
        '${err.requestOptions.method} '
        '${err.requestOptions.uri}: ${err.message}',
      );
    }
    handler.next(err);
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// ApiClient
// ─────────────────────────────────────────────────────────────────────────────

/// Singleton Dio client pre-configured with auth, retry, and logging
/// interceptors.
///
/// Usage:
/// ```dart
/// final response = await ApiClient.instance.dio.get(ApiEndpoints.feedForYou);
/// ```
class ApiClient {
  ApiClient._() {
    _storage = const FlutterSecureStorage(
      aOptions: AndroidOptions(encryptedSharedPreferences: true),
    );

    _dio = Dio(
      BaseOptions(
        baseUrl: ApiEndpoints.baseUrl,
        connectTimeout: const Duration(seconds: 60),
        receiveTimeout: const Duration(seconds: 60),
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
        },
      ),
    );

    // Interceptor order: logging fires first (outermost), then retry,
    // then auth (innermost — closest to the actual request).
    _dio.interceptors.addAll([
      _LoggingInterceptor(),
      _RetryInterceptor(dio: _dio),
      _AuthInterceptor(dio: _dio, storage: _storage),
    ]);
  }

  static final ApiClient _instance = ApiClient._();

  /// The single shared instance of [ApiClient].
  static ApiClient get instance => _instance;

  late final Dio _dio;
  late final FlutterSecureStorage _storage;

  /// The underlying [Dio] instance — use this for all HTTP calls.
  Dio get dio => _dio;

  /// Persists tokens after a successful login or register.
  Future<void> saveTokens({
    required String accessToken,
    required String refreshToken,
  }) async {
    await Future.wait([
      _storage.write(
          key: _StorageKeys.accessToken, value: accessToken),
      _storage.write(
          key: _StorageKeys.refreshToken, value: refreshToken),
    ]);
  }

  /// Wipes all stored tokens on logout.
  Future<void> clearTokens() async {
    await Future.wait([
      _storage.delete(key: _StorageKeys.accessToken),
      _storage.delete(key: _StorageKeys.refreshToken),
    ]);
  }

  /// Returns the stored access token, or null if not authenticated.
  Future<String?> getAccessToken() =>
      _storage.read(key: _StorageKeys.accessToken);

  /// Sends a fire-and-forget ping to wake the backend (Render free tier sleeps
  /// when idle). Call this at app startup so the server is ready by the time
  /// the user reaches the login/register screen.
  void warmup() {
    _dio.get<dynamic>(
      '/auth/refresh',
      options: Options(
        extra: {'skip_auth': true},
        validateStatus: (_) => true, // accept any status — we just need a round-trip
      ),
    ).catchError((_) {}); // ignore errors silently
  }
}
