import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../../../../core/error/exceptions.dart';
import '../models/user_model.dart';

abstract class AuthLocalDataSource {
  Future<void> saveTokens({required String accessToken, required String refreshToken});
  Future<String?> getAccessToken();
  Future<String?> getRefreshToken();
  Future<void> clearTokens();
  Future<void> saveUser(UserModel user);
  Future<UserModel?> getUser();
  Future<void> clearUser();
  Future<bool> hasValidSession();
}

class AuthLocalDataSourceImpl implements AuthLocalDataSource {
  static const _access  = 'tk_access';
  static const _refresh = 'tk_refresh';
  static const _user    = 'tk_user';

  static const _store = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
  );

  const AuthLocalDataSourceImpl();

  @override
  Future<void> saveTokens({required String accessToken, required String refreshToken}) async {
    try {
      await Future.wait([
        _store.write(key: _access,  value: accessToken),
        _store.write(key: _refresh, value: refreshToken),
      ]);
    } catch (e) {
      throw CacheException(message: 'saveTokens: $e');
    }
  }

  @override
  Future<String?> getAccessToken() async {
    try { return await _store.read(key: _access); } catch (_) { return null; }
  }

  @override
  Future<String?> getRefreshToken() async {
    try { return await _store.read(key: _refresh); } catch (_) { return null; }
  }

  @override
  Future<void> clearTokens() async {
    try {
      await Future.wait([_store.delete(key: _access), _store.delete(key: _refresh)]);
    } catch (e) {
      throw CacheException(message: 'clearTokens: $e');
    }
  }

  @override
  Future<void> saveUser(UserModel user) async {
    try {
      await _store.write(key: _user, value: jsonEncode(user.toJson()));
    } catch (e) {
      throw CacheException(message: 'saveUser: $e');
    }
  }

  @override
  Future<UserModel?> getUser() async {
    try {
      final raw = await _store.read(key: _user);
      if (raw == null) return null;
      return UserModel.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> clearUser() async {
    try { await _store.delete(key: _user); }
    catch (e) { throw CacheException(message: 'clearUser: $e'); }
  }

  @override
  Future<bool> hasValidSession() async {
    try {
      final t = await _store.read(key: _access);
      return t != null && t.isNotEmpty;
    } catch (_) { return false; }
  }
}