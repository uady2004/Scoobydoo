import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';

abstract interface class LikeRemoteDataSource {
  Future<bool> toggleLike(String videoId);
  Future<List<Map<String, dynamic>>> getLikedVideos({String? cursor});
  Future<bool> getLikeStatus(String videoId);
}

class LikeRemoteDataSourceImpl implements LikeRemoteDataSource {
  final Dio _dio;

  LikeRemoteDataSourceImpl(this._dio);

  @override
  Future<bool> toggleLike(String videoId) async {
    try {
      final response = await _dio.post('/videos/$videoId/like');
      return response.data['is_liked'] as bool;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<List<Map<String, dynamic>>> getLikedVideos({String? cursor}) async {
    try {
      final queryParams = <String, dynamic>{'limit': 20};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get('/me/liked-videos', queryParameters: queryParams);
      final data = response.data as Map<String, dynamic>;
      return (data['data'] as List).cast<Map<String, dynamic>>();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<bool> getLikeStatus(String videoId) async {
    try {
      final response = await _dio.get('/videos/$videoId/like-status');
      return response.data['is_liked'] as bool;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }
}
