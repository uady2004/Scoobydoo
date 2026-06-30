import 'dart:io';

import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/data/datasources/profile_remote_datasource.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

/// Concrete implementation of [ProfileRepository].
/// Delegates to [ProfileRemoteDataSource] and maps exceptions → [Failure].
class ProfileRepositoryImpl implements ProfileRepository {
  ProfileRepositoryImpl(this._remote);

  final ProfileRemoteDataSource _remote;

  // ── Profile ────────────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, ProfileEntity>> getProfile(String userId) async {
    try {
      final profile = await _remote.getProfile(userId);
      return Right(profile);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, ProfileEntity>> updateProfile(
    Map<String, dynamic> params,
  ) async {
    try {
      final updated = await _remote.updateProfile(params);
      return Right(updated);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, String>> uploadAvatar(File file) async {
    try {
      final url = await _remote.uploadAvatar(file);
      return Right(url);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  // ── Videos ─────────────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getUserVideos(String userId, String? cursor) async {
    try {
      final result = await _remote.getUserVideos(userId, cursor);
      return Right((result.$1, result.$2));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getLikedVideos(String? cursor) async {
    try {
      final result = await _remote.getLikedVideos(cursor);
      return Right((result.$1, result.$2));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getBookmarkedVideos(String? cursor) async {
    try {
      final result = await _remote.getBookmarkedVideos(cursor);
      return Right((result.$1, result.$2));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  // ── Social graph ───────────────────────────────────────────────────────────

  @override
  Future<Either<Failure, Unit>> followUser(String userId) async {
    try {
      await _remote.followUser(userId);
      return const Right(unit);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, Unit>> unfollowUser(String userId) async {
    try {
      await _remote.unfollowUser(userId);
      return const Right(unit);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } on NetworkException catch (e) {
      return Left(NetworkFailure(message: e.message));
    } catch (e) {
      return Left(UnexpectedFailure(message: e.toString()));
    }
  }
}
