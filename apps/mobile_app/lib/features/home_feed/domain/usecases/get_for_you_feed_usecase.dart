import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/feed_repository.dart';

/// Parameters for paginated feed requests.
class FeedParams extends Equatable {
  const FeedParams({this.cursor});

  /// Opaque pagination cursor; null fetches the first page.
  final String? cursor;

  @override
  List<Object?> get props => [cursor];
}

/// Retrieves the next page of the For You personalised feed.
class GetForYouFeedUseCase implements UseCase<FeedPage, FeedParams> {
  const GetForYouFeedUseCase(this._repository);

  final FeedRepository _repository;

  @override
  Future<Either<Failure, FeedPage>> call(FeedParams params) {
    return _repository.getForYouFeed(cursor: params.cursor);
  }
}
