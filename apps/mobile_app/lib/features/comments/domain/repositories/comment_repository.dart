import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';

abstract interface class CommentRepository {
  Future<Either<Failure, (List<CommentEntity>, String? nextCursor)>> getComments({
    required String videoId,
    String? cursor,
  });

  Future<Either<Failure, CommentEntity>> createComment({
    required String videoId,
    required String content,
    String? parentId,
  });

  Future<Either<Failure, bool>> likeComment(String id);

  Future<Either<Failure, Unit>> deleteComment(String id);

  Future<Either<Failure, Unit>> pinComment(String id);

  Future<Either<Failure, Unit>> reportComment({
    required String id,
    required String reason,
  });
}
