import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/core/usecases/usecase.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

class UpdateProfileParams {
  const UpdateProfileParams({
    this.displayName,
    this.username,
    this.bio,
    this.website,
    this.isPrivate,
  });

  final String? displayName;
  final String? username;
  final String? bio;
  final String? website;
  final bool? isPrivate;

  /// Converts only the non-null fields into a JSON-serialisable map
  /// so the API receives a true partial-update body.
  Map<String, dynamic> toJson() {
    return {
      if (displayName != null) 'display_name': displayName,
      if (username != null) 'username': username,
      if (bio != null) 'bio': bio,
      if (website != null) 'website': website,
      if (isPrivate != null) 'is_private': isPrivate,
    };
  }
}

class UpdateProfileUseCase
    implements UseCase<ProfileEntity, UpdateProfileParams> {
  UpdateProfileUseCase(this._repository);

  final ProfileRepository _repository;

  @override
  Future<Either<Failure, ProfileEntity>> call(
    UpdateProfileParams params,
  ) {
    return _repository.updateProfile(params.toJson());
  }
}
