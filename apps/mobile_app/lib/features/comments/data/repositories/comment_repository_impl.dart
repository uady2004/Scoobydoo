import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/comments/data/datasources/comment_remote_datasource.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';
import 'package:tiktok_clone/features/comments/domain/repositories/comment_repository.dart';

class CommentRepositoryImpl implements CommentRepository {
  final CommentRemoteDataSource _remoteDataSource;

  CommentRepositoryImpl(this._remoteDataSource);

  @override
  Future<Either<Failure, (List<CommentEntity>, String? nextCursor)>> getComments({
    required String videoId,
    String? cursor,
  }) async {
    try {
      final result = await _remoteDataSource.getComments(
        videoId: videoId,
        cursor: cursor,
      );
      return Right((result.$1, result.$2));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }

  @override
  Future<Either<Failure, CommentEntity>> createComment({
    required String videoId,
    required String content,
    String? parentId,
  }) async {
    try {
      final comment = await _remoteDataSource.createComment(
        videoId: videoId,
        content: content,
        parentId: parentId,
      );
      return Right(comment);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }

  @override
  Future<Either<Failure, bool>> likeComment(String id) async {
    try {
      final isLiked = await _remoteDataSource.likeComment(id);
      return Right(isLiked);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }

  @override
  Future<Either<Failure, Unit>> deleteComment(String id) async {
    try {
      await _remoteDataSource.deleteComment(id);
      return const Right(unit);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }

  @override
  Future<Either<Failure, Unit>> pinComment(String id) async {
    try {
      await _remoteDataSource.pinComment(id);
      return const Right(unit);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }

  @override
  Future<Either<Failure, Unit>> reportComment({
    required String id,
    required String reason,
  }) async {
    try {
      await _remoteDataSource.reportComment(id: id, reason: reason);
      return const Right(unit);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    }
  }
}
