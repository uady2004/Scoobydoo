import 'dart:io';
import 'dart:typed_data';

import 'package:fpdart/fpdart.dart';
import 'package:path/path.dart' as p;

import '../../../../core/error/failures.dart';
import '../../domain/entities/upload_progress.dart';
import '../../domain/repositories/upload_repository.dart';
import '../datasources/upload_remote_datasource.dart';

/// Chunk size: 5 MB.
const int _chunkSize = 5 * 1024 * 1024;

/// Maximum retry attempts per chunk before failing the upload.
const int _maxRetries = 3;

class UploadRepositoryImpl implements UploadRepository {
  UploadRepositoryImpl(this._remote);

  final UploadRemoteDatasource _remote;

  // -------------------------------------------------------------------------
  // Public API
  // -------------------------------------------------------------------------

  @override
  Stream<Either<Failure, UploadProgress>> uploadFile({
    required File file,
    required String mimeType,
  }) async* {
    yield* _doUpload(file: file, mimeType: mimeType);
  }

  @override
  Stream<Either<Failure, UploadProgress>> resumeUpload({
    required String uploadId,
    required File file,
    required String mimeType,
  }) async* {
    final fileSize = await file.length();
    final totalChunks = _totalChunks(fileSize);

    // Ask the server which chunks are still missing.
    final resumeResult = await _remote.resumeUpload(uploadId);
    yield* resumeResult.match(
      (failure) => Stream.value(left(failure)),
      (missingChunks) => _uploadChunks(
        file: file,
        uploadId: uploadId,
        totalChunks: totalChunks,
        fileSize: fileSize,
        chunksToUpload: missingChunks,
      ),
    );
  }

  @override
  Future<Either<Failure, UploadProgress>> checkProgress(
      String uploadId) async {
    final result = await _remote.checkUploadProgress(uploadId);
    return result.map(
      (data) => UploadProgress(
        status: _statusFromString(data['status'] as String),
        uploadId: uploadId,
        overallProgress: (data['progress'] as double),
      ),
    );
  }

  // -------------------------------------------------------------------------
  // Internal helpers
  // -------------------------------------------------------------------------

  Stream<Either<Failure, UploadProgress>> _doUpload({
    required File file,
    required String mimeType,
  }) async* {
    final fileSize = await file.length();
    final filename = p.basename(file.path);
    final totalChunks = _totalChunks(fileSize);

    // Emit initiating state.
    yield right(const UploadProgress(status: UploadStatus.initiating));

    // Initiate upload session.
    final initiateResult = await _remote.initiateUpload(
      filename: filename,
      fileSize: fileSize,
      mimeType: mimeType,
    );

    String uploadId;
    final initiateCheck = initiateResult.match(
      (f) => null,
      (id) {
        uploadId = id;
        return id;
      },
    );

    if (initiateCheck == null) {
      yield left(initiateResult.fold((f) => f, (_) => const UnexpectedFailure()));
      return;
    }

    uploadId = initiateCheck;

    yield right(UploadProgress(
      status: UploadStatus.uploading,
      uploadId: uploadId,
      totalChunks: totalChunks,
    ));

    yield* _uploadChunks(
      file: file,
      uploadId: uploadId,
      totalChunks: totalChunks,
      fileSize: fileSize,
      chunksToUpload: List.generate(totalChunks, (i) => i),
    );
  }

  Stream<Either<Failure, UploadProgress>> _uploadChunks({
    required File file,
    required String uploadId,
    required int totalChunks,
    required int fileSize,
    required List<int> chunksToUpload,
  }) async* {
    int uploadedChunks = totalChunks - chunksToUpload.length;

    final raf = await file.open();
    try {
      for (final chunkIndex in chunksToUpload) {
        final offset = chunkIndex * _chunkSize;
        final length =
            (offset + _chunkSize > fileSize) ? fileSize - offset : _chunkSize;

        await raf.setPosition(offset);
        final bytes = await raf.read(length);
        final chunkBytes = Uint8List.fromList(bytes);

        Either<Failure, double>? chunkResult;
        for (int attempt = 0; attempt < _maxRetries; attempt++) {
          chunkResult = await _remote.uploadChunk(
            uploadId: uploadId,
            chunkIndex: chunkIndex,
            chunkBytes: chunkBytes,
            onSendProgress: (sent, total) {
              // We emit inside the loop below after the await, so this is
              // intentionally left as a no-op; the outer yield handles it.
            },
          );
          if (chunkResult.isRight()) break;
          // Wait briefly before retrying (exponential back-off).
          if (attempt < _maxRetries - 1) {
            await Future<void>.delayed(
                Duration(milliseconds: 500 * (attempt + 1)));
          }
        }

        if (chunkResult == null || chunkResult.isLeft()) {
          yield left(
            chunkResult?.fold((f) => f, (_) => const UnexpectedFailure()) ??
                const UnexpectedFailure(),
          );
          return;
        }

        uploadedChunks++;
        final overallProgress = uploadedChunks / totalChunks;

        yield right(UploadProgress(
          status: UploadStatus.uploading,
          uploadId: uploadId,
          currentChunk: uploadedChunks,
          totalChunks: totalChunks,
          overallProgress: overallProgress,
        ));
      }
    } finally {
      await raf.close();
    }

    // All chunks sent — finalise.
    yield right(UploadProgress(
      status: UploadStatus.uploading,
      uploadId: uploadId,
      overallProgress: 1.0,
      currentChunk: totalChunks,
      totalChunks: totalChunks,
    ));

    final completeResult = await _remote.completeUpload(
      uploadId: uploadId,
      totalChunks: totalChunks,
    );

    yield completeResult.match(
      (failure) => left(failure),
      (videoId) => right(UploadProgress(
        status: UploadStatus.processing,
        uploadId: uploadId,
        videoId: videoId,
        overallProgress: 1.0,
        currentChunk: totalChunks,
        totalChunks: totalChunks,
      )),
    );
  }

  int _totalChunks(int fileSize) =>
      (fileSize / _chunkSize).ceil().clamp(1, 999999);

  UploadStatus _statusFromString(String s) {
    switch (s) {
      case 'processing':
        return UploadStatus.processing;
      case 'published':
        return UploadStatus.published;
      case 'failed':
        return UploadStatus.failed;
      default:
        return UploadStatus.uploading;
    }
  }
}
