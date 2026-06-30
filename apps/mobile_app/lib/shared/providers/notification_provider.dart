import 'package:flutter/foundation.dart';
import '../models/notification_model.dart';
import '../services/notification_api_service.dart';

/// NotificationProvider manages notification state for the entire app.
/// It is a ChangeNotifier and should be placed above the widget tree via
/// ChangeNotifierProvider (e.g. in main.dart with MultiProvider).
class NotificationProvider extends ChangeNotifier {
  final NotificationApiService _api;

  NotificationProvider(this._api);

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  List<NotificationModel> _notifications = [];
  int _total = 0;
  int _unreadCount = 0;
  bool _isLoading = false;
  bool _hasMore = true;
  String? _error;
  int _offset = 0;
  static const int _pageSize = 20;

  NotificationPreference? _preferences;
  bool _prefsLoading = false;

  // ---------------------------------------------------------------------------
  // Public getters
  // ---------------------------------------------------------------------------

  List<NotificationModel> get notifications => List.unmodifiable(_notifications);
  int get total => _total;
  int get unreadCount => _unreadCount;
  bool get isLoading => _isLoading;
  bool get hasMore => _hasMore;
  String? get error => _error;
  NotificationPreference? get preferences => _preferences;
  bool get prefsLoading => _prefsLoading;

  // ---------------------------------------------------------------------------
  // Notification list operations
  // ---------------------------------------------------------------------------

  /// Refresh from the top (offset = 0). Called on screen open and pull-to-refresh.
  Future<void> refresh() async {
    _offset = 0;
    _hasMore = true;
    _notifications = [];
    await _loadPage();
  }

  /// Load the next page. A no-op when already loading or no more pages exist.
  Future<void> loadMore() async {
    if (_isLoading || !_hasMore) return;
    await _loadPage();
  }

  Future<void> _loadPage() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final resp = await _api.getNotifications(
        limit: _pageSize,
        offset: _offset,
      );
      _notifications.addAll(resp.notifications);
      _total = resp.total;
      _unreadCount = resp.unreadCount;
      _offset += resp.notifications.length;
      _hasMore = _notifications.length < _total;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  /// Mark a single notification as read (optimistic update).
  Future<void> markAsRead(String notificationId) async {
    final idx = _notifications.indexWhere((n) => n.id == notificationId);
    if (idx == -1 || _notifications[idx].isRead) return;

    // Optimistic update.
    _notifications[idx] = _notifications[idx].copyWith(
      isRead: true,
      readAt: DateTime.now(),
    );
    if (_unreadCount > 0) _unreadCount--;
    notifyListeners();

    try {
      await _api.markAsRead(notificationId);
    } catch (e) {
      // Roll back on failure.
      _notifications[idx] = _notifications[idx].copyWith(isRead: false);
      if (_unreadCount < _total) _unreadCount++;
      _error = e.toString();
      notifyListeners();
    }
  }

  /// Mark all notifications as read (optimistic update).
  Future<void> markAllRead() async {
    final updated = _notifications
        .map((n) => n.isRead ? n : n.copyWith(isRead: true, readAt: DateTime.now()))
        .toList();
    _notifications = updated;
    _unreadCount = 0;
    notifyListeners();

    try {
      await _api.markAllRead();
    } catch (e) {
      _error = e.toString();
      // Re-fetch to restore true server state.
      await refresh();
    }
  }

  // ---------------------------------------------------------------------------
  // Device management
  // ---------------------------------------------------------------------------

  /// Register an FCM push token with the backend.
  Future<void> registerDevice({
    required String token,
    required String platform,
    String? appVersion,
    String? deviceName,
  }) async {
    try {
      await _api.registerDevice(
        token: token,
        platform: platform,
        appVersion: appVersion,
        deviceName: deviceName,
      );
    } catch (e) {
      _error = e.toString();
      notifyListeners();
    }
  }

  /// Unregister a push token on sign-out.
  Future<void> unregisterDevice(String token) async {
    try {
      await _api.unregisterDevice(token);
    } catch (e) {
      _error = e.toString();
      notifyListeners();
    }
  }

  // ---------------------------------------------------------------------------
  // Preferences
  // ---------------------------------------------------------------------------

  Future<void> loadPreferences() async {
    _prefsLoading = true;
    notifyListeners();
    try {
      _preferences = await _api.getPreferences();
    } catch (e) {
      _error = e.toString();
    } finally {
      _prefsLoading = false;
      notifyListeners();
    }
  }

  Future<void> updatePreferences(Map<String, dynamic> changes) async {
    try {
      _preferences = await _api.updatePreferences(changes);
      notifyListeners();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
    }
  }

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  void clearError() {
    _error = null;
    notifyListeners();
  }
}
