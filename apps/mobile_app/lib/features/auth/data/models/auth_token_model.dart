import '../../domain/entities/auth_token.dart';

class AuthTokenModel extends AuthToken {
  const AuthTokenModel({
    required super.accessToken,
    required super.refreshToken,
    required super.expiresAt,
  });

  factory AuthTokenModel.fromJson(Map<String, dynamic> json) {
    final DateTime expiresAt;
    if (json['expires_at'] != null) {
      expiresAt = DateTime.parse(json['expires_at'] as String);
    } else if (json['expires_in'] != null) {
      expiresAt = DateTime.now().add(Duration(seconds: json['expires_in'] as int));
    } else {
      // Backend issues 24-hour tokens; default to 24h so users aren't force-logged out
      expiresAt = DateTime.now().add(const Duration(hours: 24));
    }

    return AuthTokenModel(
      accessToken: (json['access_token'] ?? json['accessToken']) as String,
      refreshToken: (json['refresh_token'] ?? json['refreshToken']) as String,
      expiresAt: expiresAt,
    );
  }

  Map<String, dynamic> toJson() => {
        'access_token': accessToken,
        'refresh_token': refreshToken,
        'expires_at': expiresAt.toIso8601String(),
      };

  factory AuthTokenModel.fromEntity(AuthToken token) => AuthTokenModel(
        accessToken: token.accessToken,
        refreshToken: token.refreshToken,
        expiresAt: token.expiresAt,
      );
}
