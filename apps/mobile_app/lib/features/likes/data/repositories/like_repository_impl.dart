import 'package:fpdart/fpdart.dart';
import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/likes/domain/entities/like_entity.dart';
import 'package:tiktok_clone/features/likes/domain/repositories/like_repository.dart';
import '../models/like_model.dart';

class LikeRepositoryImpl implements LikeRepository {
  LikeRepositoryImpl(this._dio);
  final Dio _dio;

  @override
  Future<Either<Failure, LikeEntity>> toggleLike(String videoId) async {
    try {
      final r = await _dio.post<Map<String, dynamic>>('/videos/$videoId/like');
      return Right(LikeModel.fromJson(r.data!));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    } on DioException catch (e) {
      return Left(NetworkFailure(message: e.message ?? 'Network error'));
    }
  }

  @override
  Future<Either<Failure, bool>> isLiked(String videoId) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/videos/$videoId/liked');
      return Right(r.data?['is_liked'] as bool? ?? false);
    } on DioException catch (e) {
      return Left(NetworkFailure(message: e.message ?? 'Network error'));
    }
  }
}
