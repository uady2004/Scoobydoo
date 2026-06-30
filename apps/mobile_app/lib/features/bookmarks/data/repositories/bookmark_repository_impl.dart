import 'package:fpdart/fpdart.dart';
import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/bookmarks/domain/entities/bookmark_entity.dart';
import 'package:tiktok_clone/features/bookmarks/domain/repositories/bookmark_repository.dart';
import '../models/bookmark_model.dart';

class BookmarkRepositoryImpl implements BookmarkRepository {
  BookmarkRepositoryImpl(this._dio);
  final Dio _dio;

  @override
  Future<Either<Failure, bool>> toggleBookmark(String videoId) async {
    try {
      final r = await _dio.post<Map<String, dynamic>>('/videos/$videoId/bookmark');
      return Right(r.data?['is_bookmarked'] as bool? ?? false);
    } on DioException catch (e) {
      return Left(NetworkFailure(message: e.message ?? 'Network error'));
    }
  }

  @override
  Future<Either<Failure, List<BookmarkEntity>>> getBookmarks({String? cursor}) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/users/me/bookmarks',
          queryParameters: {if (cursor != null) 'cursor': cursor});
      final list = r.data?['bookmarks'] as List? ?? [];
      return Right(list.map((e) => BookmarkModel.fromJson(e as Map<String, dynamic>)).toList());
    } on DioException catch (e) {
      return Left(NetworkFailure(message: e.message ?? 'Network error'));
    }
  }

  @override
  Future<Either<Failure, bool>> isBookmarked(String videoId) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/videos/$videoId/bookmarked');
      return Right(r.data?['is_bookmarked'] as bool? ?? false);
    } on DioException catch (e) {
      return Left(NetworkFailure(message: e.message ?? 'Network error'));
    }
  }
}
