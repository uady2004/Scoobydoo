import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Abstract interface
// ─────────────────────────────────────────────────────────────────────────────

abstract interface class ReportRemoteDatasource {
  /// Submits a user report for a piece of content.
  /// Returns the server-assigned report ID on success.
  Future<String> submitReport({
    required String contentType,
    required String contentId,
    required String reason,
    String? details,
  });
}

// ─────────────────────────────────────────────────────────────────────────────
// Implementation
// ─────────────────────────────────────────────────────────────────────────────

class ReportRemoteDatasourceImpl implements ReportRemoteDatasource {
  ReportRemoteDatasourceImpl(this._dio);

  final Dio _dio;

  @override
  Future<String> submitReport({
    required String contentType,
    required String contentId,
    required String reason,
    String? details,
  }) async {
    final response = await _dio.post<Map<String, dynamic>>(
      '/reports',
      data: {
        'content_type': contentType,
        'content_id': contentId,
        'reason': reason,
        if (details != null && details.isNotEmpty) 'details': details,
      },
    );
    return (response.data?['report_id'] as String?) ?? '';
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Riverpod providers
// ─────────────────────────────────────────────────────────────────────────────

/// Internal Dio provider scoped to the reports feature.
/// Uses the shared [ApiClient] singleton so auth interceptors apply.
final _reportDioProvider = Provider<Dio>((ref) => ApiClient.instance.dio);

/// Public provider for [ReportRemoteDatasource].
final reportDatasourceProvider = Provider<ReportRemoteDatasource>((ref) {
  return ReportRemoteDatasourceImpl(ref.watch(_reportDioProvider));
});
