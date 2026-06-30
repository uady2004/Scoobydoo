/// Thrown when the remote API returns a non-2xx response.
class ServerException implements Exception {
  final String message;
  final int? statusCode;

  const ServerException({required this.message, this.statusCode});

  @override
  String toString() => 'ServerException($statusCode): $message';
}

/// Thrown when reading / writing to local secure storage fails.
class CacheException implements Exception {
  final String message;

  const CacheException({required this.message});

  @override
  String toString() => 'CacheException: $message';
}

/// Thrown when there is no internet connectivity.
class NetworkException implements Exception {
  final String message;

  const NetworkException({this.message = 'No internet connection.'});

  @override
  String toString() => 'NetworkException: $message';
}

/// Thrown when the session is invalid or expired.
class AuthException implements Exception {
  final String message;
  final int? statusCode;

  const AuthException({required this.message, this.statusCode});

  @override
  String toString() => 'AuthException($statusCode): $message';
}
