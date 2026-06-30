import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/video_entity.dart';
import '../repositories/video_repository.dart';

class GetVideoUsecase {
  const GetVideoUsecase(this._repo);
  final VideoRepository _repo;

  Future<Either<Failure, VideoEntity>> call(String videoId) =>
      _repo.getVideo(videoId);
}
