import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import '../models/notification_model.dart';

abstract interface class NotificationRemoteDatasource {
  Future<List<NotificationModel>> getNotifications({String? cursor});
  Future<void> markAsRead(String id);
  Future<void> markAllAsRead();
  Future<int> getUnreadCount();
}

class NotificationRemoteDatasourceImpl implements NotificationRemoteDatasource {
  NotificationRemoteDatasourceImpl(this._dio);
  final Dio _dio;

  @override
  Future<List<NotificationModel>> getNotifications({String? cursor}) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/notifications',
          queryParameters: {if (cursor != null) 'cursor': cursor});
      final list = r.data?['notifications'] as List? ?? [];
      return list.map((e) => NotificationModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load notifications');
    }
  }

  @override
  Future<void> markAsRead(String id) async {
    try {
      await _dio.put<void>('/notifications/$id/read');
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to mark as read');
    }
  }

  @override
  Future<void> markAllAsRead() async {
    try {
      await _dio.put<void>('/notifications/read-all');
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to mark all as read');
    }
  }

  @override
  Future<int> getUnreadCount() async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/notifications/unread-count');
      return (r.data?['count'] as num? ?? 0).toInt();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to get unread count');
    }
  }
}
