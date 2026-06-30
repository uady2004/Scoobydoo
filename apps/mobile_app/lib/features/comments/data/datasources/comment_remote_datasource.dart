import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/features/comments/data/models/comment_model.dart';

abstract interface class CommentRemoteDataSource {
  Future<(List<CommentModel>, String? nextCursor)> getComments({
    required String videoId,
    String? cursor,
  });

  Future<CommentModel> createComment({
    required String videoId,
    required String content,
    String? parentId,
  });

  Future<bool> likeComment(String id);

  Future<void> deleteComment(String id);

  Future<void> pinComment(String id);

  Future<void> reportComment({
    required String id,
    required String reason,
  });
}

class CommentRemoteDataSourceImpl implements CommentRemoteDataSource {
  final Dio _dio;

  CommentRemoteDataSourceImpl(this._dio);

  @override
  Future<(List<CommentModel>, String? nextCursor)> getComments({
    required String videoId,
    String? cursor,
  }) async {
    try {
      final queryParams = <String, dynamic>{'limit': 20};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get(
        '/videos/$videoId/comments',
        queryParameters: queryParams,
      );

      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List)
          .map((e) => CommentModel.fromJson(e as Map<String, dynamic>))
          .toList();
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
  Future<CommentModel> createComment({
    required String videoId,
    required String content,
    String? parentId,
  }) async {
    try {
      final body = <String, dynamic>{
        'content': content,
        if (parentId != null) 'parent_id': parentId,
      };

      final response = await _dio.post('/videos/$videoId/comments', data: body);
      return CommentModel.fromJson(response.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<bool> likeComment(String id) async {
    try {
      final response = await _dio.post('/comments/$id/like');
      return response.data['is_liked'] as bool;
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> deleteComment(String id) async {
    try {
      await _dio.delete('/comments/$id');
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> pinComment(String id) async {
    try {
      await _dio.post('/comments/$id/pin');
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> reportComment({required String id, required String reason}) async {
    try {
      await _dio.post('/comments/$id/report', data: {'reason': reason});
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }
}
