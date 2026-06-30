import 'package:dio/dio.dart';
import '../models/notification_model.dart';

/// NotificationApiService wraps every REST endpoint exposed by the
/// Go notification-service.
class NotificationApiService {
  final Dio _dio;

  NotificationApiService(this._dio);

  // ---------------------------------------------------------------------------
  // Notifications
  // ---------------------------------------------------------------------------

  /// Fetch a paginated list of notifications for the authenticated user.
  Future<NotificationsResponse> getNotifications({
    int limit = 20,
    int offset = 0,
    bool unreadOnly = false,
  }) async {
    final resp = await _dio.get<Map<String, dynamic>>(
      '/notifications',
      queryParameters: {
        'limit': limit,
        'offset': offset,
        if (unreadOnly) 'unread_only': 'true',
      },
    );
    return NotificationsResponse.fromJson(resp.data!);
  }

  /// Mark a single notification as read.
  Future<void> markAsRead(String notificationId) async {
    await _dio.put<void>('/notifications/$notificationId/read');
  }

  /// Mark every notification for the authenticated user as read.
  Future<void> markAllRead() async {
    await _dio.put<void>('/notifications/read');
  }

  // ---------------------------------------------------------------------------
  // Devices
  // ---------------------------------------------------------------------------

  /// Register an FCM push token for the current device.
  Future<void> registerDevice({
    required String token,
    required String platform, // 'ios' | 'android' | 'web'
    String? appVersion,
    String? deviceName,
  }) async {
    await _dio.post<void>('/devices', data: {
      'token': token,
      'platform': platform,
      if (appVersion != null) 'app_version': appVersion,
      if (deviceName != null) 'device_name': deviceName,
    });
  }

  /// Unregister a push token (e.g. on sign-out).
  Future<void> unregisterDevice(String token) async {
    await _dio.delete<void>('/devices/$token');
  }

  // ---------------------------------------------------------------------------
  // Preferences
  // ---------------------------------------------------------------------------

  /// Retrieve the notification preferences for the authenticated user.
  Future<NotificationPreference> getPreferences() async {
    final resp = await _dio.get<Map<String, dynamic>>('/preferences');
    return NotificationPreference.fromJson(resp.data!);
  }

  /// Apply a partial update to the user's notification preferences.
  Future<NotificationPreference> updatePreferences(
    Map<String, dynamic> changes,
  ) async {
    final resp = await _dio.put<Map<String, dynamic>>(
      '/preferences',
      data: changes,
    );
    return NotificationPreference.fromJson(resp.data!);
  }
}
