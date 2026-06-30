import 'package:equatable/equatable.dart';

class AuthToken extends Equatable {
  final String accessToken;
  final String refreshToken;
  final DateTime expiresAt;

  const AuthToken({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAt,
  });

  bool get isExpired => DateTime.now().isAfter(expiresAt);

  bool get isExpiringSoon =>
      DateTime.now().isAfter(expiresAt.subtract(const Duration(minutes: 5)));

  @override
  List<Object?> get props => [accessToken, refreshToken, expiresAt];
}
