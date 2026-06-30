import 'package:fpdart/fpdart.dart';
import '../../../../core/error/failures.dart';
import '../entities/auth_token.dart';
import '../entities/user_entity.dart';

abstract class AuthRepository {
  /// Authenticate with email and password.
  Future<Either<Failure, AuthToken>> login({
    required String email,
    required String password,
  });

  /// Register a new user account.
  Future<Either<Failure, AuthToken>> register({
    required String username,
    required String email,
    required String password,
    String? phone,
  });

  /// Revoke the current session server-side and clear local tokens.
  Future<Either<Failure, Unit>> logout();

  /// Authenticate via Google OAuth; [idToken] comes from google_sign_in.
  Future<Either<Failure, AuthToken>> googleSignIn({
    required String idToken,
  });

  /// Authenticate via Apple Sign-In; [identityToken] comes from sign_in_with_apple.
  Future<Either<Failure, AuthToken>> appleSignIn({
    required String identityToken,
  });

  /// Send an OTP to the given phone number.
  Future<Either<Failure, Unit>> sendOTP({
    required String phone,
  });

  /// Verify the OTP code submitted for the given phone number.
  Future<Either<Failure, AuthToken>> verifyOTP({
    required String phone,
    required String code,
  });

  /// Trigger a password-reset email.
  Future<Either<Failure, Unit>> forgotPassword({
    required String email,
  });

  /// Complete password reset using the token from the reset email.
  Future<Either<Failure, Unit>> resetPassword({
    required String token,
    required String newPassword,
  });

  /// Verify the user's email address using the token from the verification email.
  Future<Either<Failure, Unit>> verifyEmail({
    required String token,
  });

  /// Return the currently authenticated user from local storage, if any.
  Future<Either<Failure, UserEntity?>> getCurrentUser();

  /// Refresh the access token using the stored refresh token.
  Future<Either<Failure, AuthToken>> refreshToken({
    required String refreshToken,
  });
}
