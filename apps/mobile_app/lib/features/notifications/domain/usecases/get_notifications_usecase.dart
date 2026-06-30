import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/notification_entity.dart';
import '../repositories/notification_repository.dart';

class GetNotificationsUsecase {
  const GetNotificationsUsecase(this._repo);
  final NotificationRepository _repo;
  Future<Either<Failure, List<NotificationEntity>>> call({String? cursor}) =>
      _repo.getNotifications(cursor: cursor);
}
