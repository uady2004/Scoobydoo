import 'package:dio/dio.dart';

import '../../../../core/error/exceptions.dart';
import '../../../../core/network/api_client.dart';
import '../../../../core/network/api_endpoints.dart';
import '../models/feed_item_model.dart';

/// Raw remote result before domain mapping.
class FeedResponseData {
  const FeedResponseData({required this.items, this.nextCursor});

  final List<FeedItemModel> items;
  final String? nextCursor;
}

abstract class FeedRemoteDataSource {
  Future<FeedResponseData> getForYouFeed({String? cursor});
  Future<FeedResponseData> getFollowingFeed({String? cursor});
  Future<FeedResponseData> getTrendingFeed();
  Future<void> reportView({
    required String videoId,
    required int watchDuration,
    required double completionPct,
  });
}

class FeedRemoteDataSourceImpl implements FeedRemoteDataSource {
  FeedRemoteDataSourceImpl({Dio? dio})
      : _dio = dio ?? ApiClient.instance.dio;

  final Dio _dio;

  static const int _pageLimit = 10;

  @override
  Future<FeedResponseData> getForYouFeed({String? cursor}) async {
    try {
      final queryParams = <String, dynamic>{'limit': _pageLimit};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.feedForYou,
        queryParameters: queryParams,
      );

      return _parseResponse(response.data!);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  @override
  Future<FeedResponseData> getFollowingFeed({String? cursor}) async {
    try {
      final queryParams = <String, dynamic>{'limit': _pageLimit};
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.feedFollowing,
        queryParameters: queryParams,
      );

      return _parseResponse(response.data!);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  @override
  Future<FeedResponseData> getTrendingFeed() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        ApiEndpoints.feedTrending,
      );
      return _parseResponse(response.data!);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  @override
  Future<void> reportView({
    required String videoId,
    required int watchDuration,
    required double completionPct,
  }) async {
    try {
      await _dio.post<void>(
        '/feed/view',
        data: {
          'video_id': videoId,
          'watch_duration': watchDuration,
          'completion_pct': completionPct,
        },
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  // ── Helpers ──────────────────────────────────────────────────────────────

  FeedResponseData _parseResponse(Map<String, dynamic> data) {
    final rawItems = data['items'] as List<dynamic>? ?? [];
    final items = rawItems
        .map((e) => FeedItemModel.fromJson(e as Map<String, dynamic>))
        .toList();
    final nextCursor = data['next_cursor'] as String?;
    return FeedResponseData(items: items, nextCursor: nextCursor);
  }

  Exception _mapDioError(DioException e) {
    if (e.type == DioExceptionType.connectionError) {
      return const NetworkException();
    }
    final statusCode = e.response?.statusCode;
    if (statusCode == 401) {
      return AuthException(
        message: 'Session expired. Please log in again.',
        statusCode: statusCode,
      );
    }
    return ServerException(
      message: e.response?.data?['message'] as String? ??
          e.message ??
          'Unknown server error',
      statusCode: statusCode,
    );
  }
}
