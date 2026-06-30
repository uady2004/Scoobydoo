import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/like_entity.dart';

abstract interface class LikeRepository {
  Future<Either<Failure, LikeEntity>> toggleLike(String videoId);
  Future<Either<Failure, bool>> isLiked(String videoId);
}
