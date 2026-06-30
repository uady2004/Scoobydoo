import 'dart:io';

import 'package:dio/dio.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../network/api_client.dart';
import '../network/api_endpoints.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Background message handler — must be a top-level function.
// ─────────────────────────────────────────────────────────────────────────────

@pragma('vm:entry-point')
Future<void> _firebaseBackgroundHandler(RemoteMessage message) async {
  // Firebase is already initialised by the time this runs.
  debugPrint('[FCM] Background message: ${message.messageId}');
}

// ─────────────────────────────────────────────────────────────────────────────
// Android notification channel
// ─────────────────────────────────────────────────────────────────────────────

const _androidChannel = AndroidNotificationChannel(
  'tiktok_clone_default',
  'TikTok Clone Notifications',
  description: 'Likes, comments, follows, and live alerts.',
  importance: Importance.high,
  playSound: true,
  enableVibration: true,
);

// ─────────────────────────────────────────────────────────────────────────────
// Navigation callback typedef
// ─────────────────────────────────────────────────────────────────────────────

/// Called when a notification tap should route the user somewhere.
/// Receives the raw notification payload so the caller (router) can decode it.
typedef NotificationTapCallback = void Function(String? payload);

// ─────────────────────────────────────────────────────────────────────────────
// PushNotificationService
// ─────────────────────────────────────────────────────────────────────────────

/// Manages Firebase Cloud Messaging and flutter_local_notifications.
///
/// Call [initialize] once at app startup (after Firebase.initializeApp).
/// Inject [onNotificationTap] to wire navigation.
///
/// ```dart
/// await PushNotificationService.instance.initialize(
///   onNotificationTap: (payload) {
///     if (payload != null) router.go(payload);
///   },
/// );
/// ```
class PushNotificationService {
  PushNotificationService._();
  static final PushNotificationService instance = PushNotificationService._();

  final FirebaseMessaging _fcm = FirebaseMessaging.instance;
  final FlutterLocalNotificationsPlugin _localNotifications =
      FlutterLocalNotificationsPlugin();

  NotificationTapCallback? _onNotificationTap;

  // ── Public API ─────────────────────────────────────────────────────────────

  /// Bootstraps FCM, local notifications, and all message listeners.
  Future<void> initialize({
    NotificationTapCallback? onNotificationTap,
  }) async {
    _onNotificationTap = onNotificationTap;

    // Register background handler (must be top-level fn).
    FirebaseMessaging.onBackgroundMessage(_firebaseBackgroundHandler);

    await _requestPermission();
    await _setupLocalNotifications();
    await _registerToken();

    _listenForeground();
    _listenNotificationOpened();
    await _checkInitialMessage();
  }

  // ── Permission ─────────────────────────────────────────────────────────────

  Future<void> _requestPermission() async {
    final settings = await _fcm.requestPermission(
      alert: true,
      badge: true,
      sound: true,
      provisional: false,
    );
    debugPrint(
      '[FCM] Permission: ${settings.authorizationStatus}',
    );
  }

  // ── Local notifications setup ──────────────────────────────────────────────

  Future<void> _setupLocalNotifications() async {
    const androidInit = AndroidInitializationSettings('@mipmap/ic_launcher');
    const darwinInit = DarwinInitializationSettings(
      requestAlertPermission: false, // already requested via FCM
      requestBadgePermission: false,
      requestSoundPermission: false,
    );
    const initSettings = InitializationSettings(
      android: androidInit,
      iOS: darwinInit,
    );

    await _localNotifications.initialize(
      initSettings,
      onDidReceiveNotificationResponse: (details) {
        _onNotificationTap?.call(details.payload);
      },
    );

    // Create the Android high-priority channel.
    if (Platform.isAndroid) {
      await _localNotifications
          .resolvePlatformSpecificImplementation<
              AndroidFlutterLocalNotificationsPlugin>()
          ?.createNotificationChannel(_androidChannel);
    }
  }

  // ── FCM token registration ─────────────────────────────────────────────────

  Future<void> _registerToken() async {
    // APNs token must be set before FCM token on iOS.
    if (Platform.isIOS) {
      await _fcm.getAPNSToken();
    }

    final token = await _fcm.getToken();
    if (token != null) {
      await _sendTokenToBackend(token);
    }

    // Listen for token refreshes (rotations).
    _fcm.onTokenRefresh.listen(_sendTokenToBackend);
  }

  Future<void> _sendTokenToBackend(String token) async {
    try {
      await ApiClient.instance.dio.post<void>(
        ApiEndpoints.notifications,
        data: {'fcm_token': token, 'platform': Platform.operatingSystem},
      );
      debugPrint('[FCM] Token registered with backend');
    } on DioException catch (e) {
      debugPrint('[FCM] Token registration failed: ${e.message}');
    }
  }

  // ── Message listeners ──────────────────────────────────────────────────────

  /// Shows a local notification when the app is in the foreground.
  void _listenForeground() {
    FirebaseMessaging.onMessage.listen((RemoteMessage message) {
      debugPrint('[FCM] Foreground message: ${message.messageId}');
      _showLocalNotification(message);
    });
  }

  /// Handles taps on notifications that opened the app from background.
  void _listenNotificationOpened() {
    FirebaseMessaging.onMessageOpenedApp.listen((RemoteMessage message) {
      debugPrint('[FCM] Notification opened app: ${message.messageId}');
      final route = _routeFromMessage(message);
      _onNotificationTap?.call(route);
    });
  }

  /// Handles the notification that launched the app from terminated state.
  Future<void> _checkInitialMessage() async {
    final initial = await _fcm.getInitialMessage();
    if (initial != null) {
      debugPrint('[FCM] Initial message: ${initial.messageId}');
      final route = _routeFromMessage(initial);
      _onNotificationTap?.call(route);
    }
  }

  // ── Local notification display ─────────────────────────────────────────────

  Future<void> _showLocalNotification(RemoteMessage message) async {
    final notification = message.notification;
    if (notification == null) return;

    final androidDetails = AndroidNotificationDetails(
      _androidChannel.id,
      _androidChannel.name,
      channelDescription: _androidChannel.description,
      importance: Importance.high,
      priority: Priority.high,
      icon: '@mipmap/ic_launcher',
    );
    const darwinDetails = DarwinNotificationDetails(
      presentAlert: true,
      presentBadge: true,
      presentSound: true,
    );
    final details = NotificationDetails(
      android: androidDetails,
      iOS: darwinDetails,
    );

    final payload = _routeFromMessage(message);

    await _localNotifications.show(
      message.hashCode,
      notification.title,
      notification.body,
      details,
      payload: payload,
    );
  }

  // ── Route extraction ───────────────────────────────────────────────────────

  /// Extracts a go_router-compatible route string from the message data.
  /// Convention: message.data['route'] = '/video/abc123'
  String? _routeFromMessage(RemoteMessage message) {
    final data = message.data;
    if (data.containsKey('route')) return data['route'] as String?;

    // Fallback: reconstruct route from type + id fields.
    final type = data['type'] as String?;
    final id = data['id'] as String?;
    if (type != null && id != null) {
      return switch (type) {
        'video' => '/video/$id',
        'profile' => '/profile/$id',
        'comment' => '/video/${data['videoId']}/comments',
        'live' => '/live/$id',
        _ => null,
      };
    }
    return null;
  }

  /// Returns the raw JSON payload of the last notification that opened the
  /// app, or null. Useful for deep-link initialisation.
  Future<Map<String, dynamic>?> getInitialNotificationData() async {
    final message = await _fcm.getInitialMessage();
    return message?.data;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Riverpod provider
// ─────────────────────────────────────────────────────────────────────────────

/// Provides the singleton [PushNotificationService].
///
/// Initialize the service in your app entry point before accessing this
/// provider.
final pushNotificationServiceProvider = Provider<PushNotificationService>(
  (ref) => PushNotificationService.instance,
);
