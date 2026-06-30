import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../repositories/notification_repository.dart';

class MarkNotificationReadUsecase {
  const MarkNotificationReadUsecase(this._repo);
  final NotificationRepository _repo;
  Future<Either<Failure, void>> call(String id) => _repo.markAsRead(id);
}

class MarkAllReadUsecase {
  const MarkAllReadUsecase(this._repo);
  final NotificationRepository _repo;
  Future<Either<Failure, void>> call() => _repo.markAllAsRead();
}
