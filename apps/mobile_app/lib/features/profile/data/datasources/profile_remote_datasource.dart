import 'dart:io';

import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/features/home_feed/data/models/feed_item_model.dart';
import 'package:tiktok_clone/features/profile/data/models/profile_model.dart';

abstract interface class ProfileRemoteDataSource {
  Future<ProfileModel> getProfile(String userId);
  Future<ProfileModel> updateProfile(Map<String, dynamic> params);
  Future<String> uploadAvatar(File file);
  Future<(List<FeedItemModel>, String? nextCursor)> getUserVideos(
      String userId, String? cursor);
  Future<(List<FeedItemModel>, String? nextCursor)> getLikedVideos(
      String? cursor);
  Future<(List<FeedItemModel>, String? nextCursor)> getBookmarkedVideos(
      String? cursor);
  Future<void> followUser(String userId);
  Future<void> unfollowUser(String userId);
}

class ProfileRemoteDataSourceImpl implements ProfileRemoteDataSource {
  ProfileRemoteDataSourceImpl(this._dio);
  final Dio _dio;
  static const int _pageSize = 30;

  // ── Profile ────────────────────────────────────────────────────────────────

  @override
  Future<ProfileModel> getProfile(String userId) async {
    try {
      // Try /users/:id/profile first (real microservice path)
      final response = await _dio.get<Map<String, dynamic>>(
        '/users/$userId/profile',
      );
      return _parseProfile(response.data!, userId);
    } on DioException {
      try {
        // Fallback to /users/:id (simple backend path)
        final response = await _dio.get<Map<String, dynamic>>(
          '/users/$userId',
        );
        return _parseProfile(response.data!, userId);
      } on DioException {
        // Last fallback — /users/me (own profile)
        try {
          final response =
              await _dio.get<Map<String, dynamic>>('/users/me');
          return _parseProfile(response.data!, userId);
        } on DioException {
          // Return empty profile so UI never stays stuck
          return _emptyProfile(userId);
        }
      }
    }
  }

  /// Parse profile from API response — handles both backends.
  ProfileModel _parseProfile(
      Map<String, dynamic> data, String fallbackId) {
    // Normalise: ensure user_id is present
    if (data['user_id'] == null && data['id'] != null) {
      data['user_id'] = data['id'].toString();
    }
    if (data['user_id'] == null) {
      data['user_id'] = fallbackId;
    }
    return ProfileModel.fromJson(data);
  }

  /// Empty profile used when all API calls fail.
  ProfileModel _emptyProfile(String userId) {
    return ProfileModel(
      userId: userId,
      username: userId.length > 20 ? userId.substring(0, 8) : userId,
      displayName: 'User',
      bio: null,
      avatarUrl: null,
      website: null,
      followerCount: 0,
      followingCount: 0,
      likeCount: 0,
      videoCount: 0,
      isVerified: false,
      isCreator: false,
      isPrivate: false,
      createdAt: DateTime.now(),
    );
  }

  @override
  Future<ProfileModel> updateProfile(Map<String, dynamic> params) async {
    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '/users/me',
        data: params,
      );
      return _parseProfile(response.data!, '');
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<String> uploadAvatar(File file) async {
    try {
      final formData = FormData.fromMap({
        'avatar': await MultipartFile.fromFile(
          file.path,
          filename: file.uri.pathSegments.last,
        ),
      });
      final response = await _dio.post<Map<String, dynamic>>(
        '/users/me/avatar',
        data: formData,
        options: Options(contentType: 'multipart/form-data'),
      );
      final avatarUrl = response.data?['avatar_url'] as String?;
      if (avatarUrl == null || avatarUrl.isEmpty) {
        throw const ServerException(
          message: 'Avatar upload succeeded but no URL returned.',
          statusCode: 200,
        );
      }
      return avatarUrl;
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  // ── Videos ─────────────────────────────────────────────────────────────────

  @override
  Future<(List<FeedItemModel>, String? nextCursor)> getUserVideos(
    String userId,
    String? cursor,
  ) async {
    try {
      final params = <String, dynamic>{'limit': _pageSize};
      if (cursor != null) params['cursor'] = cursor;
      final response = await _dio.get<Map<String, dynamic>>(
        '/users/$userId/videos',
        queryParameters: params,
      );
      return _parsePaginatedVideos(response.data!);
    } on DioException {
      return (const <FeedItemModel>[], null);
    }
  }

  @override
  Future<(List<FeedItemModel>, String? nextCursor)> getLikedVideos(
    String? cursor,
  ) async {
    try {
      final params = <String, dynamic>{'limit': _pageSize};
      if (cursor != null) params['cursor'] = cursor;
      final response = await _dio.get<Map<String, dynamic>>(
        '/me/liked-videos',
        queryParameters: params,
      );
      return _parsePaginatedVideos(response.data!);
    } on DioException {
      return (const <FeedItemModel>[], null);
    }
  }

  @override
  Future<(List<FeedItemModel>, String? nextCursor)> getBookmarkedVideos(
    String? cursor,
  ) async {
    try {
      final params = <String, dynamic>{'limit': _pageSize};
      if (cursor != null) params['cursor'] = cursor;
      final response = await _dio.get<Map<String, dynamic>>(
        '/users/me/bookmarks',
        queryParameters: params,
      );
      return _parsePaginatedVideos(response.data!);
    } on DioException {
      return (const <FeedItemModel>[], null);
    }
  }

  // ── Social ─────────────────────────────────────────────────────────────────

  @override
  Future<void> followUser(String userId) async {
    try {
      await _dio.post<void>('/users/$userId/follow');
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  @override
  Future<void> unfollowUser(String userId) async {
    try {
      await _dio.delete<void>('/users/$userId/follow');
    } on DioException catch (e) {
      throw _mapDioException(e);
    }
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  (List<FeedItemModel>, String? nextCursor) _parsePaginatedVideos(
    Map<String, dynamic> data,
  ) {
    final rawList = data['data'] as List<dynamic>? ?? [];
    final items = rawList
        .map((e) => FeedItemModel.fromJson(e as Map<String, dynamic>))
        .toList();
    return (items, data['next_cursor'] as String?);
  }

  ServerException _mapDioException(DioException e) {
    final message = e.response?.data is Map
        ? (e.response!.data as Map)['message'] as String? ??
            e.message ??
            'Server error'
        : e.message ?? 'Server error';
    return ServerException(
      message: message,
      statusCode: e.response?.statusCode ?? 500,
    );
  }
}