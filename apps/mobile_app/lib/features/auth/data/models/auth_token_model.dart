import '../../domain/entities/auth_token.dart';

class AuthTokenModel extends AuthToken {
  const AuthTokenModel({
    required super.accessToken,
    required super.refreshToken,
    required super.expiresAt,
  });

  factory AuthTokenModel.fromJson(Map<String, dynamic> json) {
    // Support both a pre-computed ISO timestamp and a seconds-from-now TTL.
    final DateTime expiresAt;
    if (json.containsKey('expires_at') && json['expires_at'] != null) {
      expiresAt = DateTime.parse(json['expires_at'] as String);
    } else if (json.containsKey('expires_in') && json['expires_in'] != null) {
      expiresAt = DateTime.now().add(
        Duration(seconds: json['expires_in'] as int),
      );
    } else {
      // Fallback: treat token as valid for one hour.
      expiresAt = DateTime.now().add(const Duration(hours: 1));
    }

    return AuthTokenModel(
      accessToken: json['access_token'] as String? ??
          json['accessToken'] as String,
      refreshToken: json['refresh_token'] as String? ??
          json['refreshToken'] as String,
      expiresAt: expiresAt,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'access_token': accessToken,
      'refresh_token': refreshToken,
      'expires_at': expiresAt.toIso8601String(),
    };
  }

  factory AuthTokenModel.fromEntity(AuthToken token) {
    return AuthTokenModel(
      accessToken: token.accessToken,
      refreshToken: token.refreshToken,
      expiresAt: token.expiresAt,
    );
  }
}
