import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';

abstract interface class SearchRemoteDataSource {
  Future<(List<Map<String, dynamic>>, String? nextCursor)> searchAll({
    required String q,
    String? type,
    String? cursor,
  });

  Future<List<Map<String, dynamic>>> getTrendingSearches();

  Future<List<String>> getSuggestions(String q);

  Future<void> saveSearchHistory(String q);

  Future<List<String>> getSearchHistory();

  Future<void> clearHistory();
}

class SearchRemoteDataSourceImpl implements SearchRemoteDataSource {
  final Dio _dio;

  SearchRemoteDataSourceImpl(this._dio);

  @override
  Future<(List<Map<String, dynamic>>, String? nextCursor)> searchAll({
    required String q,
    String? type,
    String? cursor,
  }) async {
    try {
      final queryParams = <String, dynamic>{'q': q, 'limit': 20};
      if (type != null) queryParams['type'] = type;
      if (cursor != null) queryParams['cursor'] = cursor;

      final response = await _dio.get('/search', queryParameters: queryParams);
      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List).cast<Map<String, dynamic>>();
      final nextCursor = data['next_cursor'] as String?;
      return (items, nextCursor);
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<List<Map<String, dynamic>>> getTrendingSearches() async {
    try {
      final response = await _dio.get('/search/trending');
      final data = response.data as Map<String, dynamic>;
      return (data['data'] as List).cast<Map<String, dynamic>>();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<List<String>> getSuggestions(String q) async {
    try {
      final response = await _dio.get(
        '/search/suggestions',
        queryParameters: {'q': q},
      );
      final data = response.data as Map<String, dynamic>;
      return (data['suggestions'] as List).cast<String>();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> saveSearchHistory(String q) async {
    try {
      await _dio.post('/search/history', data: {'query': q});
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<List<String>> getSearchHistory() async {
    try {
      final response = await _dio.get('/search/history');
      final data = response.data as Map<String, dynamic>;
      return (data['history'] as List).cast<String>();
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }

  @override
  Future<void> clearHistory() async {
    try {
      await _dio.delete('/search/history');
    } on DioException catch (e) {
      throw ServerException(
        message: e.response?.data?['message'] as String? ?? e.message ?? 'Server error',
        statusCode: e.response?.statusCode ?? 500,
      );
    }
  }
}
