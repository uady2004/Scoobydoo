import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/like_entity.dart';
import '../repositories/like_repository.dart';

class ToggleLikeUsecase {
  const ToggleLikeUsecase(this._repo);
  final LikeRepository _repo;
  Future<Either<Failure, LikeEntity>> call(String videoId) => _repo.toggleLike(videoId);
}
