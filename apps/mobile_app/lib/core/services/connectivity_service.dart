import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../theme/app_colors.dart';

// ─────────────────────────────────────────────────────────────────────────────
// ConnectivityService
// ─────────────────────────────────────────────────────────────────────────────

/// Wraps [Connectivity] to expose a bool stream and a synchronous snapshot.
///
/// Also shows a [SnackBar] via the provided [navigatorKey] whenever the
/// connection drops or recovers.
///
/// ```dart
/// // In your Riverpod app setup:
/// final connectivityService = ref.watch(connectivityServiceProvider);
/// final isOnline = await connectivityService.checkNow();
///
/// // React to changes:
/// connectivityService.isConnected.listen((online) {
///   if (!online) showOfflineBanner();
/// });
/// ```
class ConnectivityService {
  ConnectivityService({GlobalKey<NavigatorState>? navigatorKey})
      : _navigatorKey = navigatorKey {
    _init();
  }

  final Connectivity _connectivity = Connectivity();
  final GlobalKey<NavigatorState>? _navigatorKey;

  final StreamController<bool> _controller =
      StreamController<bool>.broadcast();

  bool _isOnline = true;
  StreamSubscription<List<ConnectivityResult>>? _subscription;

  // ── Public API ─────────────────────────────────────────────────────────────

  /// Emits `true` when connected, `false` when not.
  Stream<bool> get isConnected => _controller.stream;

  /// Synchronous snapshot of the last-known connectivity state.
  bool get isOnline => _isOnline;

  /// Disposes the underlying stream subscription. Call from your service
  /// locator / Riverpod ref.onDispose.
  void dispose() {
    _subscription?.cancel();
    _controller.close();
  }

  /// Performs an immediate connectivity check and returns the result.
  Future<bool> checkNow() async {
    final results = await _connectivity.checkConnectivity();
    final online = _isConnected(results);
    _isOnline = online;
    return online;
  }

  // ── Initialisation ─────────────────────────────────────────────────────────

  Future<void> _init() async {
    // Seed with the current state.
    final initial = await _connectivity.checkConnectivity();
    _isOnline = _isConnected(initial);
    _controller.add(_isOnline);

    // Listen for future changes.
    _subscription =
        _connectivity.onConnectivityChanged.listen(_onConnectivityChanged);
  }

  void _onConnectivityChanged(List<ConnectivityResult> results) {
    final online = _isConnected(results);
    if (online == _isOnline) return; // No state change — skip.

    _isOnline = online;
    _controller.add(online);

    if (_navigatorKey != null) {
      _showSnackBar(online);
    }
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  bool _isConnected(List<ConnectivityResult> results) {
    return results.any((r) =>
        r == ConnectivityResult.mobile ||
        r == ConnectivityResult.wifi ||
        r == ConnectivityResult.ethernet ||
        r == ConnectivityResult.vpn);
  }

  void _showSnackBar(bool online) {
    final context = _navigatorKey?.currentContext;
    if (context == null) return;

    ScaffoldMessenger.of(context).clearSnackBars();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            Icon(
              online ? Icons.wifi : Icons.wifi_off_rounded,
              color: Colors.white,
              size: 18,
            ),
            const SizedBox(width: 10),
            Text(
              online ? 'Back online' : 'No internet connection',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
        backgroundColor:
            online ? AppColors.success.withValues(alpha: 0.9) : AppColors.error,
        behavior: SnackBarBehavior.floating,
        duration: Duration(seconds: online ? 2 : 6),
        margin: const EdgeInsets.all(12),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Riverpod providers
// ─────────────────────────────────────────────────────────────────────────────

/// Provides the [ConnectivityService] singleton.
///
/// Wire up the [navigatorKey] at the provider override site if you want
/// automatic snackbar notifications:
/// ```dart
/// ProviderScope(
///   overrides: [
///     connectivityServiceProvider.overrideWithValue(
///       ConnectivityService(navigatorKey: navigatorKey),
///     ),
///   ],
/// )
/// ```
final connectivityServiceProvider = Provider<ConnectivityService>(
  (ref) {
    final service = ConnectivityService();
    ref.onDispose(service.dispose);
    return service;
  },
);

/// A [StreamProvider] that rebuilds widgets whenever connectivity changes.
///
/// ```dart
/// final isOnline = ref.watch(isConnectedProvider);
/// isOnline.when(
///   data: (online) => online ? const OnlineWidget() : const OfflineBanner(),
///   loading: () => const SizedBox.shrink(),
///   error: (_, __) => const SizedBox.shrink(),
/// );
/// ```
final isConnectedProvider = StreamProvider<bool>((ref) {
  final service = ref.watch(connectivityServiceProvider);
  return service.isConnected;
});
