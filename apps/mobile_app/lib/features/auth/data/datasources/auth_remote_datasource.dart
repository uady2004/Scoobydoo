import 'package:dio/dio.dart';
import '../../../../core/error/exceptions.dart';
import '../../../../core/network/api_client.dart';

abstract class AuthRemoteDataSource {
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  });

  Future<Map<String, dynamic>> register({
    required String username,
    required String email,
    required String password,
    String? phone,
  });

  Future<void> logout();

  Future<Map<String, dynamic>> refreshToken({
    required String refreshToken,
  });

  Future<Map<String, dynamic>> googleSignIn({required String idToken});
  Future<Map<String, dynamic>> appleSignIn({required String identityToken});
  Future<void> sendOTP({required String phone});
  Future<Map<String, dynamic>> verifyOTP({required String phone, required String code});
  Future<void> forgotPassword({required String email});
  Future<void> resetPassword({required String token, required String newPassword});
  Future<void> verifyEmail({required String token});
}

class AuthRemoteDataSourceImpl implements AuthRemoteDataSource {
  final Dio _dio;

  AuthRemoteDataSourceImpl({Dio? dio})
      : _dio = dio ?? ApiClient.instance.dio;

  // ── Helpers ───────────────────────────────────────────────────────────────

  Map<String, dynamic> _body(Response<dynamic> r) {
    if (r.data is Map<String, dynamic>) return r.data as Map<String, dynamic>;
    throw ServerException(
      message: 'Unexpected response format.',
      statusCode: r.statusCode,
    );
  }

  /// Converts DioException → typed exception. Never returns.
  Exception _mapError(DioException e) {
    final code = e.response?.statusCode;
    final msg  = _msg(e.response?.data) ?? e.message ?? 'Unknown error';

    if (e.type == DioExceptionType.connectionError) {
      return const NetworkException(message: 'No internet connection.');
    }
    if (e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.receiveTimeout) {
      return ServerException(message: 'Request timed out.', statusCode: code);
    }
    if (code == 401 || code == 403) {
      return AuthException(message: msg, statusCode: code);
    }
    return ServerException(message: msg, statusCode: code);
  }

  String? _msg(dynamic data) {
    if (data is Map) {
      return (data['message'] ?? data['error'] ?? data['msg'])?.toString();
    }
    return null;
  }

  // ── Auth ──────────────────────────────────────────────────────────────────

  @override
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  }) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/login',
        data: {'email': email, 'password': password},
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> register({
    required String username,
    required String email,
    required String password,
    String? phone,
  }) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/register',
        data: {
          'username': username,
          'email':    email,
          'password': password,
          if (phone != null && phone.isNotEmpty) 'phone': phone,
        },
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<void> logout() async {
    try {
      await _dio.post<dynamic>('/auth/logout');
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> refreshToken({
    required String refreshToken,
  }) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> googleSignIn({required String idToken}) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/oauth/google',
        data: {'id_token': idToken},
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> appleSignIn({required String identityToken}) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/oauth/apple',
        data: {'identity_token': identityToken},
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<void> sendOTP({required String phone}) async {
    try {
      await _dio.post<dynamic>('/auth/otp/send', data: {'phone': phone});
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> verifyOTP({
    required String phone,
    required String code,
  }) async {
    try {
      final r = await _dio.post<dynamic>(
        '/auth/otp/verify',
        data: {'phone': phone, 'code': code},
      );
      return _body(r);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<void> forgotPassword({required String email}) async {
    try {
      await _dio.post<dynamic>(
        '/auth/forgot-password',
        data: {'email': email},
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<void> resetPassword({
    required String token,
    required String newPassword,
  }) async {
    try {
      await _dio.post<dynamic>(
        '/auth/reset-password',
        data: {'token': token, 'new_password': newPassword},
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  @override
  Future<void> verifyEmail({required String token}) async {
    try {
      await _dio.post<dynamic>(
        '/auth/verify-email',
        data: {'token': token},
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}