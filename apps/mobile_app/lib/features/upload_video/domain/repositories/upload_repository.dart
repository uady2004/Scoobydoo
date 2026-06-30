import 'dart:io';

import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/upload_progress.dart';

abstract interface class UploadRepository {
  /// Splits [file] into 5 MB chunks and uploads them sequentially.
  /// Emits [UploadProgress] events throughout the lifecycle.
  Stream<Either<Failure, UploadProgress>> uploadFile({
    required File file,
    required String mimeType,
  });

  /// Resumes a previously interrupted upload.
  Stream<Either<Failure, UploadProgress>> resumeUpload({
    required String uploadId,
    required File file,
    required String mimeType,
  });

  /// Polls the server for a processing-stage update.
  Future<Either<Failure, UploadProgress>> checkProgress(String uploadId);
}
