import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/feed_item_entity.dart';

/// Paginated result returned by feed use-cases.
class FeedPage {
  const FeedPage({required this.items, this.nextCursor});

  final List<FeedItemEntity> items;

  /// Opaque cursor string for the next page; null when no more pages exist.
  final String? nextCursor;
}

/// Contract between domain and data layers for feed operations.
abstract class FeedRepository {
  /// Fetches the personalised For You feed.
  /// Pass [cursor] to load subsequent pages (null = first page).
  Future<Either<Failure, FeedPage>> getForYouFeed({String? cursor});

  /// Fetches the feed of creators the current user follows.
  Future<Either<Failure, FeedPage>> getFollowingFeed({String? cursor});

  /// Fetches globally trending videos.
  Future<Either<Failure, FeedPage>> getTrendingFeed();

  /// Reports a completed video view to the analytics back-end.
  ///
  /// [watchDuration] — seconds actually watched.
  /// [completionPct] — fraction 0.0–1.0 of the video that was watched.
  Future<Either<Failure, Unit>> reportView({
    required String videoId,
    required int watchDuration,
    required double completionPct,
  });
}
