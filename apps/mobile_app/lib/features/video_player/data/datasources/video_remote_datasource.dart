import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import '../models/video_model.dart';

abstract interface class VideoRemoteDatasource {
  Future<VideoModel> getVideo(String videoId);
  Future<void> reportView(String videoId, {int watchedSeconds});
}

class VideoRemoteDatasourceImpl implements VideoRemoteDatasource {
  VideoRemoteDatasourceImpl(this._dio);
  final Dio _dio;

  @override
  Future<VideoModel> getVideo(String videoId) async {
    try {
      final r =
          await _dio.get<Map<String, dynamic>>('/videos/$videoId');
      return VideoModel.fromJson(r.data!);
    } on DioException catch (e) {
      throw ServerException(
        message: e.message ?? 'Failed to load video',
        statusCode: e.response?.statusCode,
      );
    }
  }

  @override
  Future<void> reportView(String videoId, {int watchedSeconds = 0}) async {
    try {
      await _dio.post<void>(
        '/videos/$videoId/view',
        data: {'watched_seconds': watchedSeconds},
      );
    } catch (_) {
      // View reporting is fire-and-forget; failures must not
      // interrupt playback or surface to the user.
    }
  }
}
