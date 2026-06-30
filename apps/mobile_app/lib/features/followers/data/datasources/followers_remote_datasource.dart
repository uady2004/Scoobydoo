import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';

abstract interface class FollowersRemoteDataSource {
  Future<(List<Map<String, dynamic>>, String? nextCursor)> getFollowers({
    required String userId,
    String? cursor,
  });

  Future<(List<Map<String, dynamic>>, String? nextCursor)> getFollowing({
    required String userId,
    String? cursor,
  });

  Future<bool> followUser(String userId);

  Future<void> unfollowUser(String userId);
}

class FollowersRemoteDataSourceImpl implements FollowersRemoteDataSource {
  final Dio _dio;

  FollowersRemoteDataSourceImpl(this._dio);

  @override
  Future<(List<Map<String, dynamic>>, String? nextCursor)> getFollowers({
    required String userId,
    String? cursor,
  }) async {
    try {
      final queryParams = <String, dynamic>{'limit': 30};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get(
        '/users/$userId/followers',
        queryParameters: queryParams,
      );
      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List).cast<Map<String, dynamic>>();
      final nextCursor = data['next_cursor'] as String?;
      return (items, nextCursor);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<(List<Map<String, dynamic>>, String? nextCursor)> getFollowing({
    required String userId,
    String? cursor,
  }) async {
    try {
      final queryParams = <String, dynamic>{'limit': 30};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get(
        '/users/$userId/following',
        queryParameters: queryParams,
      );
      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List).cast<Map<String, dynamic>>();
      final nextCursor = data['next_cursor'] as String?;
      return (items, nextCursor);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<bool> followUser(String userId) async {
    try {
      final response = await _dio.post('/users/$userId/follow');
      return response.data['is_following'] as bool? ?? true;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> unfollowUser(String userId) async {
    try {
      await _dio.delete('/users/$userId/follow');
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }
}
