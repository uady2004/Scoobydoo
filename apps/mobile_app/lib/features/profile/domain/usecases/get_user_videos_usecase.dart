import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/core/usecases/usecase.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

class GetUserVideosParams {
  const GetUserVideosParams({
    required this.userId,
    this.cursor,
    this.tab = VideoTab.posted,
  });

  final String userId;
  final String? cursor;

  /// Which grid the caller wants to populate.
  final VideoTab tab;
}

enum VideoTab { posted, liked, bookmarked }

/// Single use-case covering the Videos / Liked / Bookmarks tabs on the
/// profile grid.  The [VideoTab] param routes to the correct repository
/// method, keeping the presentation layer free of repository knowledge.
class GetUserVideosUseCase
    implements
        UseCase<(List<FeedItemEntity>, String? nextCursor), GetUserVideosParams> {
  GetUserVideosUseCase(this._repository);

  final ProfileRepository _repository;

  @override
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>> call(
    GetUserVideosParams params,
  ) {
    switch (params.tab) {
      case VideoTab.posted:
        return _repository.getUserVideos(params.userId, params.cursor);
      case VideoTab.liked:
        return _repository.getLikedVideos(params.cursor);
      case VideoTab.bookmarked:
        return _repository.getBookmarkedVideos(params.cursor);
    }
  }
}
