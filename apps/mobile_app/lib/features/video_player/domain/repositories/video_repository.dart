import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/video_entity.dart';

abstract interface class VideoRepository {
  Future<Either<Failure, VideoEntity>> getVideo(String videoId);
  Future<Either<Failure, void>> reportView(String videoId, {int watchedSeconds});
}
