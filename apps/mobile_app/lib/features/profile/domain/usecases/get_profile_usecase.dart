import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/core/usecases/usecase.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

class GetProfileParams {
  const GetProfileParams({required this.userId});

  final String userId;
}

class GetProfileUseCase implements UseCase<ProfileEntity, GetProfileParams> {
  GetProfileUseCase(this._repository);

  final ProfileRepository _repository;

  @override
  Future<Either<Failure, ProfileEntity>> call(GetProfileParams params) {
    return _repository.getProfile(params.userId);
  }
}
