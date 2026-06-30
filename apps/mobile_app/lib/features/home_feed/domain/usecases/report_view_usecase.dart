import 'package:equatable/equatable.dart';
import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../repositories/feed_repository.dart';

class ReportViewParams extends Equatable {
  const ReportViewParams({
    required this.videoId,
    required this.watchDuration,
    required this.completionPct,
  });

  final String videoId;

  /// Seconds the user actually watched.
  final int watchDuration;

  /// Fraction 0.0–1.0 of the total video that was watched.
  final double completionPct;

  @override
  List<Object?> get props => [videoId, watchDuration, completionPct];
}

/// Reports a video view event to the analytics back-end.
class ReportViewUseCase implements UseCase<Unit, ReportViewParams> {
  const ReportViewUseCase(this._repository);

  final FeedRepository _repository;

  @override
  Future<Either<Failure, Unit>> call(ReportViewParams params) {
    return _repository.reportView(
      videoId: params.videoId,
      watchDuration: params.watchDuration,
      completionPct: params.completionPct,
    );
  }
}
