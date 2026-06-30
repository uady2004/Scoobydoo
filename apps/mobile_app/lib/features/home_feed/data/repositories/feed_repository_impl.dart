import 'package:fpdart/fpdart.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/error/failures.dart';
import '../../domain/entities/feed_item_entity.dart';
import '../../domain/repositories/feed_repository.dart';
import '../datasources/feed_remote_datasource.dart';

class FeedRepositoryImpl implements FeedRepository {
  const FeedRepositoryImpl(this._remoteDataSource);

  final FeedRemoteDataSource _remoteDataSource;

  @override
  Future<Either<Failure, FeedPage>> getForYouFeed({String? cursor}) async {
    return _safeCall(
      () => _remoteDataSource.getForYouFeed(cursor: cursor),
    );
  }

  @override
  Future<Either<Failure, FeedPage>> getFollowingFeed({String? cursor}) async {
    return _safeCall(
      () => _remoteDataSource.getFollowingFeed(cursor: cursor),
    );
  }

  @override
  Future<Either<Failure, FeedPage>> getTrendingFeed() async {
    return _safeCall(() => _remoteDataSource.getTrendingFeed());
  }

  @override
  Future<Either<Failure, Unit>> reportView({
    required String videoId,
    required int watchDuration,
    required double completionPct,
  }) async {
    try {
      await _remoteDataSource.reportView(
        videoId: videoId,
        watchDuration: watchDuration,
        completionPct: completionPct,
      );
      return right(unit);
    } on NetworkException catch (e) {
      return left(NetworkFailure(message: e.message));
    } on AuthException catch (e) {
      return left(AuthFailure(message: e.message, statusCode: e.statusCode));
    } on ServerException catch (e) {
      return left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (_) {
      return left(const UnexpectedFailure());
    }
  }

  // ── Helpers ──────────────────────────────────────────────────────────────

  Future<Either<Failure, FeedPage>> _safeCall(
    Future<FeedResponseData> Function() call,
  ) async {
    try {
      final data = await call();
      final page = FeedPage(
        items: data.items
            .map<FeedItemEntity>((m) => m.toEntity())
            .toList(),
        nextCursor: data.nextCursor,
      );
      return right(page);
    } on NetworkException catch (e) {
      return left(NetworkFailure(message: e.message));
    } on AuthException catch (e) {
      return left(AuthFailure(message: e.message, statusCode: e.statusCode));
    } on ServerException catch (e) {
      return left(ServerFailure(message: e.message, statusCode: e.statusCode));
    } catch (_) {
      return left(const UnexpectedFailure());
    }
  }
}
