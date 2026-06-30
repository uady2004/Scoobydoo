import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';
import 'package:tiktok_clone/features/comments/domain/repositories/comment_repository.dart';

class GetCommentsParams {
  final String videoId;
  final String? cursor;

  const GetCommentsParams({required this.videoId, this.cursor});
}

class GetCommentsUseCase {
  final CommentRepository _repository;

  GetCommentsUseCase(this._repository);

  Future<Either<Failure, (List<CommentEntity>, String? nextCursor)>> call(
    GetCommentsParams params,
  ) {
    return _repository.getComments(
      videoId: params.videoId,
      cursor: params.cursor,
    );
  }
}
