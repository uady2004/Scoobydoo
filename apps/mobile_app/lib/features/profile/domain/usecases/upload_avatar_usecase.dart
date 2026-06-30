import 'dart:io';

import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/core/usecases/usecase.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';

class UploadAvatarParams {
  const UploadAvatarParams({required this.file});

  final File file;
}

/// Returns the remote CDN URL of the newly uploaded avatar.
class UploadAvatarUseCase implements UseCase<String, UploadAvatarParams> {
  UploadAvatarUseCase(this._repository);

  final ProfileRepository _repository;

  @override
  Future<Either<Failure, String>> call(UploadAvatarParams params) {
    return _repository.uploadAvatar(params.file);
  }
}
