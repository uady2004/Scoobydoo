import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/notification_entity.dart';

abstract interface class NotificationRepository {
  Future<Either<Failure, List<NotificationEntity>>> getNotifications({String? cursor});
  Future<Either<Failure, void>> markAsRead(String notificationId);
  Future<Either<Failure, void>> markAllAsRead();
  Future<Either<Failure, int>> getUnreadCount();
}
