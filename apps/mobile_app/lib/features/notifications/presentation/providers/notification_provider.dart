import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/notifications/data/datasources/notification_remote_datasource.dart';
import 'package:tiktok_clone/features/notifications/data/repositories/notification_repository_impl.dart';
import 'package:tiktok_clone/features/notifications/domain/entities/notification_entity.dart';
import 'package:tiktok_clone/features/notifications/domain/usecases/get_notifications_usecase.dart';
import 'package:tiktok_clone/features/notifications/domain/usecases/mark_read_usecase.dart';

final notificationDatasourceProvider = Provider<NotificationRemoteDatasource>((ref) =>
    NotificationRemoteDatasourceImpl(ApiClient.instance.dio));

final notificationRepoProvider = Provider((ref) =>
    NotificationRepositoryImpl(ref.watch(notificationDatasourceProvider)));

final getNotificationsUsecaseProvider = Provider((ref) =>
    GetNotificationsUsecase(ref.watch(notificationRepoProvider)));

final markReadUsecaseProvider = Provider((ref) =>
    MarkNotificationReadUsecase(ref.watch(notificationRepoProvider)));

final markAllReadUsecaseProvider = Provider((ref) =>
    MarkAllReadUsecase(ref.watch(notificationRepoProvider)));

// ─── State ───────────────────────────────────────────────────────────────────

class NotificationsState {
  const NotificationsState({
    this.notifications = const [],
    this.isLoading = false,
    this.hasMore = true,
    this.cursor,
    this.error,
  });
  final List<NotificationEntity> notifications;
  final bool isLoading;
  final bool hasMore;
  final String? cursor;
  final String? error;

  NotificationsState copyWith({
    List<NotificationEntity>? notifications,
    bool? isLoading,
    bool? hasMore,
    String? cursor,
    String? error,
  }) => NotificationsState(
    notifications: notifications ?? this.notifications,
    isLoading: isLoading ?? this.isLoading,
    hasMore: hasMore ?? this.hasMore,
    cursor: cursor ?? this.cursor,
    error: error,
  );
}

class NotificationsNotifier extends StateNotifier<NotificationsState> {
  NotificationsNotifier(this._getUsecase, this._markRead, this._markAll)
      : super(const NotificationsState());

  final GetNotificationsUsecase _getUsecase;
  final MarkNotificationReadUsecase _markRead;
  final MarkAllReadUsecase _markAll;

  Future<void> load() async {
    state = state.copyWith(isLoading: true, error: null);
    final result = await _getUsecase();
    result.fold(
      (f) => state = state.copyWith(isLoading: false, error: f.toString()),
      (list) => state = state.copyWith(
        isLoading: false,
        notifications: list,
        hasMore: list.length >= 20,
      ),
    );
  }

  Future<void> loadMore() async {
    if (!state.hasMore || state.isLoading) return;
    state = state.copyWith(isLoading: true);
    final result = await _getUsecase(cursor: state.cursor);
    result.fold(
      (f) => state = state.copyWith(isLoading: false),
      (list) => state = state.copyWith(
        isLoading: false,
        notifications: [...state.notifications, ...list],
        hasMore: list.length >= 20,
      ),
    );
  }

  Future<void> markRead(String id) async {
    state = state.copyWith(
      notifications: state.notifications.map((n) => n.id == id ? n.copyWith(isRead: true) : n).toList(),
    );
    await _markRead(id);
  }

  Future<void> markAllRead() async {
    state = state.copyWith(
      notifications: state.notifications.map((n) => n.copyWith(isRead: true)).toList(),
    );
    await _markAll();
  }
}

final notificationsProvider = StateNotifierProvider<NotificationsNotifier, NotificationsState>((ref) =>
    NotificationsNotifier(
      ref.watch(getNotificationsUsecaseProvider),
      ref.watch(markReadUsecaseProvider),
      ref.watch(markAllReadUsecaseProvider),
    )..load());

final unreadCountProvider = Provider<int>((ref) {
  final state = ref.watch(notificationsProvider);
  return state.notifications.where((n) => !n.isRead).length;
});
