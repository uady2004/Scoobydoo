import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/entities/user_entity.dart';
import '../../domain/usecases/login_usecase.dart';
import '../../domain/usecases/logout_usecase.dart';
import '../../domain/usecases/register_usecase.dart';
import '../../data/datasources/auth_local_datasource.dart';
import '../../data/datasources/auth_remote_datasource.dart';
import '../../data/repositories/auth_repository_impl.dart';
import '../../../../core/usecases/usecase.dart';

// ── States ────────────────────────────────────────────────────────────────────

sealed class AuthState {
  const AuthState();
}

class AuthInitial extends AuthState {
  const AuthInitial();
}

class AuthLoading extends AuthState {
  const AuthLoading();
}

class AuthAuthenticated extends AuthState {
  final UserEntity user;
  const AuthAuthenticated(this.user);
}

class AuthUnauthenticated extends AuthState {
  const AuthUnauthenticated();
}

class AuthError extends AuthState {
  final String message;
  const AuthError(this.message);
}

// ── Internal providers ────────────────────────────────────────────────────────

final _remoteProvider = Provider<AuthRemoteDataSource>(
  (_) => AuthRemoteDataSourceImpl(),
);

final _localProvider = Provider<AuthLocalDataSource>(
  (_) => const AuthLocalDataSourceImpl(),
);

final _repoProvider = Provider<AuthRepositoryImpl>(
  (ref) => AuthRepositoryImpl(
    remote: ref.watch(_remoteProvider),
    local: ref.watch(_localProvider),
  ),
);

final loginUseCaseProvider =
    Provider((ref) => LoginUseCase(ref.watch(_repoProvider)));

final registerUseCaseProvider =
    Provider((ref) => RegisterUseCase(ref.watch(_repoProvider)));

final logoutUseCaseProvider =
    Provider((ref) => LogoutUseCase(ref.watch(_repoProvider)));

// ── Notifier ──────────────────────────────────────────────────────────────────

class AuthNotifier extends AsyncNotifier<AuthState> {
  @override
  Future<AuthState> build() => _restoreSession();

  // ── Session restore ───────────────────────────────────────────────────────

  Future<AuthState> _restoreSession() async {
    try {
      final local = ref.read(_localProvider);
      if (!await local.hasValidSession()) return const AuthUnauthenticated();
      final user = await local.getUser();
      if (user != null) return AuthAuthenticated(user);
    } catch (_) {}
    return const AuthUnauthenticated();
  }

  // ── Login ─────────────────────────────────────────────────────────────────

  Future<void> login({
    required String email,
    required String password,
  }) async {
    state = const AsyncData(AuthLoading());
    try {
      final result = await ref.read(loginUseCaseProvider).call(
            LoginParams(email: email, password: password),
          );
      state = await result.fold(
        (failure) async => AsyncData(AuthError(failure.message)),
        (_) async {
          final user = await ref.read(_localProvider).getUser();
          if (user != null) return AsyncData(AuthAuthenticated(user));
          // Fallback: backend saved token but not user object
          return AsyncData(AuthAuthenticated(_tempUser(email)));
        },
      );
    } catch (e) {
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
              email: email,
              password: password,
              phone: phone,
            ),
          );
      state = await result.fold(
        (failure) async => AsyncData(AuthError(failure.message)),
        (_) async {
          // Backend returns tokens on register — auto-login the user
          final user = await ref.read(_localProvider).getUser();
          if (user != null) return AsyncData(AuthAuthenticated(user));
          return AsyncData(AuthAuthenticated(_tempUser(email, username: username)));
        },
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
    state = const AsyncData(AuthUnauthenticated());
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  void clearError() {
    if (state.value is AuthError) {
      state = const AsyncData(AuthUnauthenticated());
    }
  }

  UserEntity _tempUser(String email, {String? username}) {
    final name = username ?? email.split('@').first;
    return _TempUser(username: name, email: email);
  }
}

/// Minimal user created when secure storage read fails after a successful auth.
class _TempUser extends UserEntity {
  _TempUser({required String username, required String email})
      : super(
          id: 'tmp_$email',
          username: username,
          displayName: username,
          email: email,
          isVerified: false,
          isCreator: false,
          createdAt: DateTime.now(),
        );
}

final authProvider =
    AsyncNotifierProvider<AuthNotifier, AuthState>(AuthNotifier.new);
