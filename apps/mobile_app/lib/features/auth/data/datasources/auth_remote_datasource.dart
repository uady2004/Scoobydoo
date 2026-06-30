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

  Future<Map<String, dynamic>> googleSignIn({
    required String idToken,
  });

  Future<Map<String, dynamic>> appleSignIn({
    required String identityToken,
  });

  Future<void> sendOTP({
    required String phone,
  });

  Future<Map<String, dynamic>> verifyOTP({
    required String phone,
    required String code,
  });

  Future<void> forgotPassword({
    required String email,
  });

  Future<void> resetPassword({
    required String token,
    required String newPassword,
  });

  Future<void> verifyEmail({
    required String token,
  });
}

class AuthRemoteDataSourceImpl implements AuthRemoteDataSource {
  final Dio _dio;

  AuthRemoteDataSourceImpl({Dio? dio})
      : _dio = dio ?? ApiClient.instance.dio;

  // -------------------------------------------------------------------------
  // Internal helpers
  // -------------------------------------------------------------------------

  Map<String, dynamic> _body(Response<dynamic> response) {
    final data = response.data;
    if (data is Map<String, dynamic>) return data;
    throw ServerException(
      message: 'Unexpected response format from server.',
      statusCode: response.statusCode,
    );
  }

  Never _handleDioError(DioException e) {
    final statusCode = e.response?.statusCode;
    final serverMessage = _extractServerMessage(e.response?.data);

    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        throw ServerException(
          message: 'Request timed out. Please try again.',
          statusCode: statusCode,
        );
      case DioExceptionType.badResponse:
        if (statusCode == 401 || statusCode == 403) {
          throw AuthException(
            message: serverMessage ?? 'Authentication failed.',
            statusCode: statusCode,
          );
        }
        throw ServerException(
          message: serverMessage ?? 'Server error ($statusCode).',
          statusCode: statusCode,
        );
      case DioExceptionType.connectionError:
        throw const NetworkException(
          message: 'Cannot reach the server. Check your connection.',
        );
      default:
        throw ServerException(
          message: serverMessage ?? e.message ?? 'An unknown error occurred.',
          statusCode: statusCode,
        );
    }
  }

  String? _extractServerMessage(dynamic data) {
    if (data is Map<String, dynamic>) {
      return data['message'] as String? ??
          data['error'] as String? ??
          data['msg'] as String?;
    }
    return null;
  }

  // -------------------------------------------------------------------------
  // Auth endpoints
  // -------------------------------------------------------------------------

  @override
  Future<Map<String, dynamic>> login({
    required String email,
    required String password,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/login',
        data: {'email': email, 'password': password},
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
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
      final response = await _dio.post(
        '/auth/register',
        data: {
          'username': username,
          'email': email,
          'password': password,
          if (phone != null) 'phone': phone,
        },
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<void> logout() async {
    try {
      await _dio.post('/auth/logout');
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> refreshToken({
    required String refreshToken,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> googleSignIn({
    required String idToken,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/oauth/google',
        data: {'id_token': idToken},
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> appleSignIn({
    required String identityToken,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/oauth/apple',
        data: {'identity_token': identityToken},
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<void> sendOTP({required String phone}) async {
    try {
      await _dio.post('/auth/otp/send', data: {'phone': phone});
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<Map<String, dynamic>> verifyOTP({
    required String phone,
    required String code,
  }) async {
    try {
      final response = await _dio.post(
        '/auth/otp/verify',
        data: {'phone': phone, 'code': code},
      );
      return _body(response);
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<void> forgotPassword({required String email}) async {
    try {
      await _dio.post('/auth/forgot-password', data: {'email': email});
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<void> resetPassword({
    required String token,
    required String newPassword,
  }) async {
    try {
      await _dio.post(
        '/auth/reset-password',
        data: {'token': token, 'new_password': newPassword},
      );
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }

  @override
  Future<void> verifyEmail({required String token}) async {
    try {
      await _dio.post('/auth/verify-email', data: {'token': token});
    } on DioException catch (e) {
      _handleDioError(e);
    }
  }
}
