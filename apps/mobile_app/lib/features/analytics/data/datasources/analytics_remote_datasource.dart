import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/core/network/api_endpoints.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Models
// ─────────────────────────────────────────────────────────────────────────────

class DateCountPoint {
  const DateCountPoint({required this.date, required this.count});
  final String date;
  final double count;

  factory DateCountPoint.fromJson(Map<String, dynamic> j) =>
      DateCountPoint(date: j['date'] as String, count: (j['count'] as num).toDouble());
}

class DateAmountPoint {
  const DateAmountPoint({required this.date, required this.amount});
  final String date;
  final double amount;

  factory DateAmountPoint.fromJson(Map<String, dynamic> j) =>
      DateAmountPoint(date: j['date'] as String, amount: (j['amount'] as num).toDouble());
}

class TopVideo {
  const TopVideo({
    required this.id,
    required this.title,
    required this.views,
    required this.likes,
    this.thumbnailUrl,
  });
  final String id;
  final String title;
  final int views;
  final int likes;
  final String? thumbnailUrl;

  factory TopVideo.fromJson(Map<String, dynamic> j) => TopVideo(
        id: j['id'] as String,
        title: j['title'] as String,
        views: (j['views'] as num).toInt(),
        likes: (j['likes'] as num).toInt(),
        thumbnailUrl: j['thumbnail_url'] as String?,
      );
}

class TrafficSources {
  const TrafficSources({
    required this.fyp,
    required this.following,
    required this.profile,
    required this.search,
    required this.other,
  });
  final double fyp;
  final double following;
  final double profile;
  final double search;
  final double other;

  factory TrafficSources.fromJson(Map<String, dynamic> j) => TrafficSources(
        fyp: (j['fyp'] as num).toDouble(),
        following: (j['following'] as num).toDouble(),
        profile: (j['profile'] as num).toDouble(),
        search: (j['search'] as num).toDouble(),
        other: (j['other'] as num).toDouble(),
      );
}

class Demographics {
  const Demographics({
    required this.ages,
    required this.genders,
    required this.countries,
  });
  final Map<String, double> ages;
  final Map<String, double> genders;
  final Map<String, double> countries;

  factory Demographics.fromJson(Map<String, dynamic> j) => Demographics(
        ages: _toDoubleMap(j['ages'] as Map<String, dynamic>),
        genders: _toDoubleMap(j['genders'] as Map<String, dynamic>),
        countries: _toDoubleMap(j['countries'] as Map<String, dynamic>),
      );

  static Map<String, double> _toDoubleMap(Map<String, dynamic> m) =>
      m.map((k, v) => MapEntry(k, (v as num).toDouble()));
}

class AnalyticsData {
  const AnalyticsData({
    required this.totalViews,
    required this.viewsData,
    required this.followerGrowth,
    required this.engagementRate,
    required this.topVideos,
    required this.revenueByDay,
    required this.trafficSources,
    required this.demographics,
    required this.newFollowers,
    required this.totalRevenue,
  });

  final int totalViews;
  final List<DateCountPoint> viewsData;
  final List<DateCountPoint> followerGrowth;
  final double engagementRate;
  final List<TopVideo> topVideos;
  final List<DateAmountPoint> revenueByDay;
  final TrafficSources trafficSources;
  final Demographics demographics;
  final int newFollowers;
  final double totalRevenue;

  factory AnalyticsData.fromJson(Map<String, dynamic> j) => AnalyticsData(
        totalViews: (j['totalViews'] as num).toInt(),
        viewsData: (j['viewsData'] as List)
            .map((e) => DateCountPoint.fromJson(e as Map<String, dynamic>))
            .toList(),
        followerGrowth: (j['followerGrowth'] as List)
            .map((e) => DateCountPoint.fromJson(e as Map<String, dynamic>))
            .toList(),
        engagementRate: (j['engagementRate'] as num).toDouble(),
        topVideos: (j['topVideos'] as List)
            .map((e) => TopVideo.fromJson(e as Map<String, dynamic>))
            .toList(),
        revenueByDay: (j['revenueByDay'] as List)
            .map((e) => DateAmountPoint.fromJson(e as Map<String, dynamic>))
            .toList(),
        trafficSources: TrafficSources.fromJson(
            j['trafficSources'] as Map<String, dynamic>),
        demographics:
            Demographics.fromJson(j['demographics'] as Map<String, dynamic>),
        newFollowers: (j['newFollowers'] as num).toInt(),
        totalRevenue: (j['totalRevenue'] as num).toDouble(),
      );
}

class VideoAnalyticsData {
  const VideoAnalyticsData({
    required this.videoId,
    required this.views,
    required this.likes,
    required this.comments,
    required this.shares,
    required this.averageWatchTime,
    required this.completionRate,
    required this.viewsData,
    required this.trafficSources,
  });

  final String videoId;
  final int views;
  final int likes;
  final int comments;
  final int shares;
  final double averageWatchTime;
  final double completionRate;
  final List<DateCountPoint> viewsData;
  final TrafficSources trafficSources;

  factory VideoAnalyticsData.fromJson(Map<String, dynamic> j) =>
      VideoAnalyticsData(
        videoId: j['videoId'] as String,
        views: (j['views'] as num).toInt(),
        likes: (j['likes'] as num).toInt(),
        comments: (j['comments'] as num).toInt(),
        shares: (j['shares'] as num).toInt(),
        averageWatchTime: (j['averageWatchTime'] as num).toDouble(),
        completionRate: (j['completionRate'] as num).toDouble(),
        viewsData: (j['viewsData'] as List)
            .map((e) => DateCountPoint.fromJson(e as Map<String, dynamic>))
            .toList(),
        trafficSources: TrafficSources.fromJson(
            j['trafficSources'] as Map<String, dynamic>),
      );
}

// ─────────────────────────────────────────────────────────────────────────────
// Datasource interface
// ─────────────────────────────────────────────────────────────────────────────

abstract interface class AnalyticsRemoteDataSource {
  /// Fetches creator-level analytics for [period] ('7d' | '28d' | '90d').
  Future<AnalyticsData> getCreatorAnalytics({required String period});

  /// Fetches per-video analytics for [videoId] and [period].
  Future<VideoAnalyticsData> getVideoAnalytics({
    required String videoId,
    required String period,
  });
}

// ─────────────────────────────────────────────────────────────────────────────
// Implementation
// ─────────────────────────────────────────────────────────────────────────────

class AnalyticsRemoteDataSourceImpl implements AnalyticsRemoteDataSource {
  AnalyticsRemoteDataSourceImpl(this._client);

  final ApiClient _client;

  @override
  Future<AnalyticsData> getCreatorAnalytics({required String period}) async {
    try {
      final response = await _client.dio.get<Map<String, dynamic>>(
        ApiEndpoints.analyticsCreator,
        queryParameters: {'period': period},
      );
      return AnalyticsData.fromJson(response.data!);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            e.message ??
            'Failed to fetch creator analytics',
        statusCode: e.response?.statusCode ?? 500,
      );
    } catch (e) {
      throw ServerException(message: e.toString());
    }
  }

  @override
  Future<VideoAnalyticsData> getVideoAnalytics({
    required String videoId,
    required String period,
  }) async {
    try {
      final response = await _client.dio.get<Map<String, dynamic>>(
        ApiEndpoints.analyticsVideo(videoId),
        queryParameters: {'period': period},
      );
      return VideoAnalyticsData.fromJson(response.data!);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ??
            e.message ??
            'Failed to fetch video analytics',
        statusCode: e.response?.statusCode ?? 500,
      );
    } catch (e) {
      throw ServerException(message: e.toString());
    }
  }
}
