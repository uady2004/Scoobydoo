import 'package:equatable/equatable.dart';

enum UploadStatus {
  idle,
  initiating,
  uploading,
  processing,
  published,
  failed,
}

class UploadProgress extends Equatable {
  const UploadProgress({
    required this.status,
    this.uploadId,
    this.videoId,
    this.chunkProgress = 0.0,
    this.overallProgress = 0.0,
    this.currentChunk = 0,
    this.totalChunks = 0,
    this.errorMessage,
  });

  final UploadStatus status;
  final String? uploadId;
  final String? videoId;

  /// Progress of the current chunk being sent (0.0 – 1.0).
  final double chunkProgress;

  /// Cumulative progress across all chunks (0.0 – 1.0).
  final double overallProgress;

  final int currentChunk;
  final int totalChunks;
  final String? errorMessage;

  UploadProgress copyWith({
    UploadStatus? status,
    String? uploadId,
    String? videoId,
    double? chunkProgress,
    double? overallProgress,
    int? currentChunk,
    int? totalChunks,
    String? errorMessage,
  }) {
    return UploadProgress(
      status: status ?? this.status,
      uploadId: uploadId ?? this.uploadId,
      videoId: videoId ?? this.videoId,
      chunkProgress: chunkProgress ?? this.chunkProgress,
      overallProgress: overallProgress ?? this.overallProgress,
      currentChunk: currentChunk ?? this.currentChunk,
      totalChunks: totalChunks ?? this.totalChunks,
      errorMessage: errorMessage ?? this.errorMessage,
    );
  }

  @override
  List<Object?> get props => [
        status,
        uploadId,
        videoId,
        chunkProgress,
        overallProgress,
        currentChunk,
        totalChunks,
        errorMessage,
      ];
}
