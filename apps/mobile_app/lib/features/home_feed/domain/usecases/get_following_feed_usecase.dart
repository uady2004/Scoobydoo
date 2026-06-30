import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/feed_repository.dart';
import 'get_for_you_feed_usecase.dart';

/// Retrieves the next page of the Following feed.
class GetFollowingFeedUseCase implements UseCase<FeedPage, FeedParams> {
  const GetFollowingFeedUseCase(this._repository);

  final FeedRepository _repository;

  @override
  Future<Either<Failure, FeedPage>> call(FeedParams params) {
    return _repository.getFollowingFeed(cursor: params.cursor);
  }
}
