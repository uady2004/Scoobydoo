import 'package:fpdart/fpdart.dart';
import 'package:google_sign_in/google_sign_in.dart';
import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/auth_token.dart';
import '../repositories/auth_repository.dart';

class GoogleSignInUseCase implements UseCase<AuthToken, NoParams> {
  final AuthRepository _repository;
  final GoogleSignIn _googleSignIn;

  GoogleSignInUseCase({
    required AuthRepository repository,
    GoogleSignIn? googleSignIn,
  })  : _repository = repository,
        _googleSignIn = googleSignIn ??
            GoogleSignIn(scopes: ['email', 'profile']);

  @override
  Future<Either<Failure, AuthToken>> call(NoParams params) async {
    try {
      // Trigger the Google sign-in flow.
      final googleUser = await _googleSignIn.signIn();
      if (googleUser == null) {
        // User cancelled the sign-in dialog.
        return left(
          const AuthFailure(message: 'Google sign-in was cancelled.'),
        );
      }

      final googleAuth = await googleUser.authentication;
      final idToken = googleAuth.idToken;

      if (idToken == null) {
        return left(
          const AuthFailure(
            message: 'Failed to obtain Google ID token.',
          ),
        );
      }

      return _repository.googleSignIn(idToken: idToken);
    } catch (e) {
      return left(
        AuthFailure(message: 'Google sign-in failed: ${e.toString()}'),
      );
    }
  }
}
