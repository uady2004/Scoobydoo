import 'package:fpdart/fpdart.dart';
import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/auth_token.dart';
import '../../domain/entities/user_entity.dart';
import '../../domain/repositories/auth_repository.dart';
import '../datasources/auth_local_datasource.dart';
import '../datasources/auth_remote_datasource.dart';
import '../models/auth_token_model.dart';
import '../models/user_model.dart';

class AuthRepositoryImpl implements AuthRepository {
  const AuthRepositoryImpl({
    required AuthRemoteDataSource remote,
    required AuthLocalDataSource local,
  })  : _remote = remote,
        _local = local;

  final AuthRemoteDataSource _remote;
  final AuthLocalDataSource _local;

  // ── Sanitise user JSON from any backend shape ─────────────────────────────
  static UserModel _toUser(Map<String, dynamic> raw) {
    final username = (raw['username'] ?? '').toString();
    return UserModel(
      id: (raw['id'] ?? raw['user_id'] ?? raw['uid'] ?? '').toString(),
      username: username,
      displayName:
          (raw['display_name'] ?? raw['displayName'] ?? username).toString(),
      email: (raw['email'] ?? '').toString(),
      phone: raw['phone']?.toString(),
      avatarUrl: (raw['avatar_url'] ?? raw['avatarUrl'])?.toString(),
      isVerified: (raw['email_verified'] ??
              raw['is_verified'] ??
              false) as bool,
      isCreator: (raw['is_creator'] ?? false) as bool,
      createdAt: DateTime.tryParse(
              (raw['created_at'] ?? raw['createdAt'] ?? '').toString()) ??
          DateTime.now(),
    );
  }

  // ── Save tokens + user, return token ─────────────────────────────────────
  Future<AuthToken> _persist(Map<String, dynamic> data) async {
    // Support { tokens: {...}, user: {...} } or flat { access_token, user }
    final td = data['tokens'] as Map<String, dynamic>? ?? data;
    final token = AuthTokenModel.fromJson(td);
    await _local.saveTokens(
      accessToken: token.accessToken,
      refreshToken: token.refreshToken,
    );
    final rawUser = data['user'] as Map<String, dynamic>?;
    if (rawUser != null) {
      try { await _local.saveUser(_toUser(rawUser)); } catch (_) {}
    }
    return token;
  }

  Failure _fail(Object e) {
    if (e is AuthException)    return AuthFailure(message: e.message, statusCode: e.statusCode);
    if (e is ServerException)  return ServerFailure(message: e.message, statusCode: e.statusCode);
    if (e is NetworkException) return const NetworkFailure();
    if (e is CacheException)   return CacheFailure(message: e.message);
    return UnexpectedFailure(message: e.toString());
  }

  @override
  Future<Either<Failure, AuthToken>> login({
    required String email,
    required String password,
  }) async {
    try {
      return right(await _persist(
          await _remote.login(email: email, password: password)));
    } catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, AuthToken>> register({
    required String username,
    required String email,
    required String password,
    String? phone,
  }) async {
    try {
      return right(await _persist(await _remote.register(
        username: username, email: email,
        password: password, phone: phone,
      )));
    } catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, Unit>> logout() async {
    try { await _remote.logout(); } catch (_) {}
    try {
      await Future.wait([_local.clearTokens(), _local.clearUser()]);
    } catch (_) {}
    return right(unit);
  }

  @override
  Future<Either<Failure, UserEntity?>> getCurrentUser() async {
    try { return right(await _local.getUser()); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, AuthToken>> googleSignIn({required String idToken}) async {
    try { return right(await _persist(await _remote.googleSignIn(idToken: idToken))); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, AuthToken>> appleSignIn({required String identityToken}) async {
    try { return right(await _persist(await _remote.appleSignIn(identityToken: identityToken))); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, Unit>> sendOTP({required String phone}) async {
    try { await _remote.sendOTP(phone: phone); return right(unit); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, AuthToken>> verifyOTP({required String phone, required String code}) async {
    try { return right(await _persist(await _remote.verifyOTP(phone: phone, code: code))); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, Unit>> forgotPassword({required String email}) async {
    try { await _remote.forgotPassword(email: email); return right(unit); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, Unit>> resetPassword({required String token, required String newPassword}) async {
    try { await _remote.resetPassword(token: token, newPassword: newPassword); return right(unit); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, Unit>> verifyEmail({required String token}) async {
    try { await _remote.verifyEmail(token: token); return right(unit); }
    catch (e) { return left(_fail(e)); }
  }

  @override
  Future<Either<Failure, AuthToken>> refreshToken({required String refreshToken}) async {
    try {
      final data = await _remote.refreshToken(refreshToken: refreshToken);
      final token = AuthTokenModel.fromJson(data);
      await _local.saveTokens(
        accessToken: token.accessToken,
        refreshToken: token.refreshToken,
      );
      return right(token);
    } catch (e) { return left(_fail(e)); }
  }
}