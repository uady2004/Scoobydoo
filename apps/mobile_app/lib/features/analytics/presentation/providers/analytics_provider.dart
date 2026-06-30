import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/analytics/data/datasources/analytics_remote_datasource.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure providers
// ─────────────────────────────────────────────────────────────────────────────

final analyticsDataSourceProvider =
    Provider<AnalyticsRemoteDataSource>((ref) {
  return AnalyticsRemoteDataSourceImpl(ApiClient.instance);
});

// ─────────────────────────────────────────────────────────────────────────────
// State notifier
// ─────────────────────────────────────────────────────────────────────────────

class AnalyticsNotifier extends AsyncNotifier<AnalyticsData> {
  late String _period;

  @override
  Future<AnalyticsData> build() async {
    _period = '7d';
    return _fetch();
  }

  Future<AnalyticsData> _fetch() {
    final ds = ref.read(analyticsDataSourceProvider);
    return ds.getCreatorAnalytics(period: _period);
  }

  /// Switch the active period and reload all chart data.
  Future<void> setPeriod(String period) async {
    if (_period == period) return;
    _period = period;
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Public provider
// ─────────────────────────────────────────────────────────────────────────────

/// Parameterised by period so the period chip wires directly to provider args.
/// Usage: `ref.watch(analyticsProvider('7d'))`
final analyticsProvider =
    AsyncNotifierProvider<AnalyticsNotifier, AnalyticsData>(
  AnalyticsNotifier.new,
);

// ─────────────────────────────────────────────────────────────────────────────
// Per-video analytics provider
// ─────────────────────────────────────────────────────────────────────────────

final videoAnalyticsProvider = FutureProvider.family<VideoAnalyticsData,
    ({String videoId, String period})>((ref, args) {
  final ds = ref.read(analyticsDataSourceProvider);
  return ds.getVideoAnalytics(videoId: args.videoId, period: args.period);
});
