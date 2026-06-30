import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/core/usecases/usecase.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

class FollowUserParams {
  const FollowUserParams({required this.userId, required this.isFollowing});

  /// The ID of the user to follow or unfollow.
  final String userId;

  /// If [isFollowing] is true the action is an unfollow; otherwise follow.
  /// The use-case dispatches the correct repository method based on this flag.
  final bool isFollowing;
}

class FollowUserUseCase implements UseCase<Unit, FollowUserParams> {
  FollowUserUseCase(this._repository);

  final ProfileRepository _repository;

  @override
  Future<Either<Failure, Unit>> call(FollowUserParams params) {
    if (params.isFollowing) {
      return _repository.unfollowUser(params.userId);
    }
    return _repository.followUser(params.userId);
  }
}
