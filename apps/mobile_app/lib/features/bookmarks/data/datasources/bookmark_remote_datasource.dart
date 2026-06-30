import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';

abstract interface class BookmarkRemoteDataSource {
  Future<bool> toggleBookmark(String videoId);
  Future<(List<Map<String, dynamic>>, String? nextCursor)> getBookmarkedVideos({
    String? cursor,
  });
}

class BookmarkRemoteDataSourceImpl implements BookmarkRemoteDataSource {
  final Dio _dio;

  BookmarkRemoteDataSourceImpl(this._dio);

  @override
  Future<bool> toggleBookmark(String videoId) async {
    try {
      final response = await _dio.post('/videos/$videoId/bookmark');
      return response.data['is_bookmarked'] as bool;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<(List<Map<String, dynamic>>, String? nextCursor)> getBookmarkedVideos({
    String? cursor,
  }) async {
    try {
      final queryParams = <String, dynamic>{'limit': 30};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response =
          await _dio.get('/me/bookmarks', queryParameters: queryParams);
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
}
