import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';
import 'package:tiktok_clone/features/comments/domain/repositories/comment_repository.dart';

class CreateCommentParams {
  final String videoId;
  final String content;
  final String? parentId;

  const CreateCommentParams({
    required this.videoId,
    required this.content,
    this.parentId,
  });
}

class CreateCommentUseCase {
  final CommentRepository _repository;

  CreateCommentUseCase(this._repository);

  Future<Either<Failure, CommentEntity>> call(CreateCommentParams params) {
    return _repository.createComment(
      videoId: params.videoId,
      content: params.content,
      parentId: params.parentId,
    );
  }
}
