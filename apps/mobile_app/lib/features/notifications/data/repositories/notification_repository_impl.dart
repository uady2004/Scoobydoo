import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/notifications/domain/entities/notification_entity.dart';
import 'package:tiktok_clone/features/notifications/domain/repositories/notification_repository.dart';
import '../datasources/notification_remote_datasource.dart';

class NotificationRepositoryImpl implements NotificationRepository {
  const NotificationRepositoryImpl(this._ds);
  final NotificationRemoteDatasource _ds;

  @override
  Future<Either<Failure, List<NotificationEntity>>> getNotifications({String? cursor}) async {
    try {
      return Right(await _ds.getNotifications(cursor: cursor));
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }

  @override
  Future<Either<Failure, void>> markAsRead(String id) async {
    try {
      await _ds.markAsRead(id);
      return const Right(null);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }

  @override
  Future<Either<Failure, void>> markAllAsRead() async {
    try {
      await _ds.markAllAsRead();
      return const Right(null);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }

  @override
  Future<Either<Failure, int>> getUnreadCount() async {
    try {
      return Right(await _ds.getUnreadCount());
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }
}
