import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/entities/user_entity.dart';
import '../../domain/usecases/login_usecase.dart';
import '../../domain/usecases/logout_usecase.dart';
import '../../domain/usecases/register_usecase.dart';
import '../../domain/usecases/google_signin_usecase.dart';
import '../../data/datasources/auth_local_datasource.dart';
import '../../data/datasources/auth_remote_datasource.dart';
import '../../data/repositories/auth_repository_impl.dart';
import '../../../../core/usecases/usecase.dart';

// ── States ────────────────────────────────────────────────────────────────────

sealed class AuthState { const AuthState(); }
class AuthUnauthenticated extends AuthState { const AuthUnauthenticated(); }
class AuthLoading         extends AuthState { const AuthLoading(); }
class AuthRegistered      extends AuthState {
  final String email;
  const AuthRegistered(this.email);
}
class AuthAuthenticated extends AuthState {
  final UserEntity user;
  const AuthAuthenticated(this.user);
}
class AuthError extends AuthState {
  final String message;
  const AuthError(this.message);
}

// ── Providers ─────────────────────────────────────────────────────────────────

final _remoteProvider = Provider<AuthRemoteDataSource>(
  (_) => AuthRemoteDataSourceImpl(),
);
final _localProvider = Provider<AuthLocalDataSource>(
  (_) => const AuthLocalDataSourceImpl(),
);
final _repoProvider = Provider<AuthRepositoryImpl>(
  (ref) => AuthRepositoryImpl(
    remote: ref.watch(_remoteProvider),
    local:  ref.watch(_localProvider),
  ),
);
final loginUseCaseProvider    = Provider((ref) => LoginUseCase(ref.watch(_repoProvider)));
final registerUseCaseProvider = Provider((ref) => RegisterUseCase(ref.watch(_repoProvider)));
final logoutUseCaseProvider   = Provider((ref) => LogoutUseCase(ref.watch(_repoProvider)));
final googleSignInUseCaseProvider = Provider(
  (ref) => GoogleSignInUseCase(repository: ref.watch(_repoProvider)),
);

// ── Notifier ──────────────────────────────────────────────────────────────────

class AuthNotifier extends AsyncNotifier<AuthState> {
  @override
Future<AuthState> build() async {
  // Always start fresh — clear any stuck loading state
  return _restore();
}

  // ── Restore session on app start ──────────────────────────────────────────
  Future<AuthState> _restore() async {
    try {
      final local = ref.read(_localProvider);
      if (!await local.hasValidSession()) return const AuthUnauthenticated();
      final user = await local.getUser();
      if (user != null) return AuthAuthenticated(user);
      return const AuthUnauthenticated();
    } catch (_) {
      return const AuthUnauthenticated();
    }
  }

  // ── Login ─────────────────────────────────────────────────────────────────
  Future<void> login({
    required String email,
    required String password,
  }) async {
    // Set loading
    state = const AsyncData(AuthLoading());

    try {
      final result = await ref.read(loginUseCaseProvider).call(
        LoginParams(email: email, password: password),
      );

      // Always update state — never stay on AuthLoading
      state = await result.fold(
        (failure) async => AsyncData(AuthError(failure.message)),
        (_) async {
          final user = await ref.read(_localProvider).getUser();
          if (user != null) return AsyncData(AuthAuthenticated(user));
          // Fallback — create minimal user from email
          return AsyncData(AuthAuthenticated(_TempUser(email: email)));
        },
      );
    } catch (e) {
      // Catch any unexpected error — never stay on loading
      state = AsyncData(AuthError(e.toString()));
    }
  }

  // ── Register ──────────────────────────────────────────────────────────────
  Future<void> register({
    required String username,
    required String email,
    required String password,
    String? phone,
  }) async {
    state = const AsyncData(AuthLoading());

    try {
      final result = await ref.read(registerUseCaseProvider).call(
        RegisterParams(
          username: username,
          email:    email,
          password: password,
          phone:    phone,
        ),
      );

      // Always update state
      state = result.fold(
        (failure) => AsyncData(AuthError(failure.message)),
        (_)       => AsyncData(AuthRegistered(email)),
      );
    } catch (e) {
      state = AsyncData(AuthError(e.toString()));
    }
  }

  // ── Google ────────────────────────────────────────────────────────────────
  Future<void> googleSignIn() async {
    state = const AsyncData(AuthLoading());
    try {
      final result = await ref.read(googleSignInUseCaseProvider).call(const NoParams());
      state = await result.fold(
        (f) async => AsyncData(AuthError(f.message)),
        (_) async => AsyncData(await _restore()),
      );
    } catch (e) {
      state = AsyncData(AuthError(e.toString()));
    }
  }

  // ── Apple ─────────────────────────────────────────────────────────────────
  Future<void> appleSignIn({required String identityToken}) async {
    state = const AsyncData(AuthLoading());
    try {
      final result = await ref.read(_repoProvider)
          .appleSignIn(identityToken: identityToken);
      state = await result.fold(
        (f) async => AsyncData(AuthError(f.message)),
        (_) async => AsyncData(await _restore()),
      );
    } catch (e) {
      state = AsyncData(AuthError(e.toString()));
    }
  }

  // ── Logout ────────────────────────────────────────────────────────────────
  Future<void> logout() async {
    state = const AsyncData(AuthLoading());
    try {
      await ref.read(logoutUseCaseProvider).call(const NoParams());
    } catch (_) {}
    // Always go to unauthenticated
    state = const AsyncData(AuthUnauthenticated());
  }

  void clearError() {
    if (state.value is AuthError) {
      state = const AsyncData(AuthUnauthenticated());
    }
  }
}

/// Temporary user when storage fails — prevents infinite loading.
class _TempUser extends UserEntity {
  _TempUser({required String email})
      : super(
          id:          'tmp_${DateTime.now().millisecondsSinceEpoch}',
          username:    email.split('@').first,
          displayName: email.split('@').first,
          email:       email,
          isVerified:  false,
          isCreator:   false,
          createdAt:   DateTime.now(),
        );
}

final authProvider =
    AsyncNotifierProvider<AuthNotifier, AuthState>(AuthNotifier.new);