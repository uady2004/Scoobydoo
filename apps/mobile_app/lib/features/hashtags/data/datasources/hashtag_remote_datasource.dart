import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import '../models/hashtag_model.dart';

abstract interface class HashtagRemoteDatasource {
  Future<HashtagModel> getHashtag(String tag);
  Future<List<HashtagModel>> getTrendingHashtags();
}

class HashtagRemoteDatasourceImpl implements HashtagRemoteDatasource {
  HashtagRemoteDatasourceImpl(this._dio);
  final Dio _dio;

  @override
  Future<HashtagModel> getHashtag(String tag) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/hashtags/$tag');
      return HashtagModel.fromJson(r.data!);
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load hashtag');
    }
  }

  @override
  Future<List<HashtagModel>> getTrendingHashtags() async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/hashtags/trending');
      final list = r.data?['hashtags'] as List? ?? [];
      return list.map((e) => HashtagModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load trending hashtags');
    }
  }
}
