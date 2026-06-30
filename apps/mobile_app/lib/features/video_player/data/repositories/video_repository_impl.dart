import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/video_player/domain/entities/video_entity.dart';
import 'package:tiktok_clone/features/video_player/domain/repositories/video_repository.dart';
import '../datasources/video_remote_datasource.dart';

class VideoRepositoryImpl implements VideoRepository {
  VideoRepositoryImpl(this._ds);
  final VideoRemoteDatasource _ds;

  @override
  Future<Either<Failure, VideoEntity>> getVideo(String videoId) async {
    try {
      return Right(await _ds.getVideo(videoId));
    } on ServerException catch (e) {
      return Left(
        ServerFailure(
          message: e.message,
          statusCode: e.statusCode ?? 500,
        ),
      );
    }
  }

  @override
  Future<Either<Failure, void>> reportView(
    String videoId, {
    int watchedSeconds = 0,
  }) async {
    await _ds.reportView(videoId, watchedSeconds: watchedSeconds);
    return const Right(null);
  }
}
