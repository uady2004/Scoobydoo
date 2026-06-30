import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/network/api_client.dart';

abstract interface class UploadRemoteDatasource {
  /// Initiates a new chunked upload session.
  /// Returns an uploadId that identifies the session server-side.
  Future<Either<Failure, String>> initiateUpload({
    required String filename,
    required int fileSize,
    required String mimeType,
  });

  /// Uploads a single chunk.
  /// Returns a double in [0.0, 1.0] representing overall upload progress.
  Future<Either<Failure, double>> uploadChunk({
    required String uploadId,
    required int chunkIndex,
    required Uint8List chunkBytes,
    void Function(int sent, int total)? onSendProgress,
  });

  /// Finalises the upload once all chunks have been sent.
  /// Returns the server-assigned videoId.
  Future<Either<Failure, String>> completeUpload({
    required String uploadId,
    required int totalChunks,
  });

  /// Asks the server which chunks are still missing so the client can resume.
  Future<Either<Failure, List<int>>> resumeUpload(String uploadId);

  /// Returns {progress: double, status: String} for an ongoing upload.
  Future<Either<Failure, Map<String, dynamic>>> checkUploadProgress(
      String uploadId);
}

class UploadRemoteDatasourceImpl implements UploadRemoteDatasource {
  UploadRemoteDatasourceImpl(this._client);

  final ApiClient _client;

  @override
  Future<Either<Failure, String>> initiateUpload({
    required String filename,
    required int fileSize,
    required String mimeType,
  }) async {
    try {
      final response = await _client.dio.post<Map<String, dynamic>>(
        '/uploads/initiate',
        data: {
          'filename': filename,
          'file_size': fileSize,
          'mime_type': mimeType,
        },
      );
      final uploadId = response.data!['upload_id'] as String;
      return right(uploadId);
    } on DioException catch (e) {
      return left(ServerFailure(message: e.message ?? 'Upload initiation failed'));
    } catch (e) {
      return left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, double>> uploadChunk({
    required String uploadId,
    required int chunkIndex,
    required Uint8List chunkBytes,
    void Function(int sent, int total)? onSendProgress,
  }) async {
    try {
      final formData = FormData.fromMap({
        'upload_id': uploadId,
        'chunk_index': chunkIndex.toString(),
        'chunk': MultipartFile.fromBytes(
          chunkBytes,
          filename: 'chunk_$chunkIndex',
        ),
      });

      final response = await _client.dio.post<Map<String, dynamic>>(
        '/uploads/chunk',
        data: formData,
        onSendProgress: onSendProgress,
      );

      final progress = (response.data!['progress'] as num).toDouble();
      return right(progress);
    } on DioException catch (e) {
      return left(ServerFailure(message: e.message ?? 'Chunk upload failed'));
    } catch (e) {
      return left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, String>> completeUpload({
    required String uploadId,
    required int totalChunks,
  }) async {
    try {
      final response = await _client.dio.post<Map<String, dynamic>>(
        '/uploads/complete',
        data: {'upload_id': uploadId, 'total_chunks': totalChunks},
      );
      final videoId = response.data!['video_id'] as String;
      return right(videoId);
    } on DioException catch (e) {
      return left(ServerFailure(message: e.message ?? 'Upload completion failed'));
    } catch (e) {
      return left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, List<int>>> resumeUpload(String uploadId) async {
    try {
      final response = await _client.dio.get<Map<String, dynamic>>(
        '/uploads/resume',
        queryParameters: {'upload_id': uploadId},
      );
      final missing = (response.data!['missing_chunks'] as List)
          .map((e) => e as int)
          .toList();
      return right(missing);
    } on DioException catch (e) {
      return left(ServerFailure(message: e.message ?? 'Resume upload failed'));
    } catch (e) {
      return left(UnexpectedFailure(message: e.toString()));
    }
  }

  @override
  Future<Either<Failure, Map<String, dynamic>>> checkUploadProgress(
      String uploadId) async {
    try {
      final response = await _client.dio.get<Map<String, dynamic>>(
        '/uploads/progress',
        queryParameters: {'upload_id': uploadId},
      );
      return right({
        'progress': (response.data!['progress'] as num).toDouble(),
        'status': response.data!['status'] as String,
      });
    } on DioException catch (e) {
      return left(ServerFailure(message: e.message ?? 'Progress check failed'));
    } catch (e) {
      return left(UnexpectedFailure(message: e.toString()));
    }
  }
}
