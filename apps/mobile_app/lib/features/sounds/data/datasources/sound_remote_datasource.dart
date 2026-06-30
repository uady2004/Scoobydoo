import 'package:dio/dio.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import '../models/sound_model.dart';

abstract interface class SoundRemoteDatasource {
  Future<SoundModel> getSound(String soundId);
  Future<List<SoundModel>> getTrendingSounds({int limit});
  Future<List<SoundModel>> searchSounds(String query);
}

class SoundRemoteDatasourceImpl implements SoundRemoteDatasource {
  SoundRemoteDatasourceImpl(this._dio);
  final Dio _dio;

  @override
  Future<SoundModel> getSound(String soundId) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/sounds/$soundId');
      return SoundModel.fromJson(r.data!);
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load sound');
    }
  }

  @override
  Future<List<SoundModel>> getTrendingSounds({int limit = 20}) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/sounds/trending', queryParameters: {'limit': limit});
      final list = r.data?['sounds'] as List? ?? [];
      return list.map((e) => SoundModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to load sounds');
    }
  }

  @override
  Future<List<SoundModel>> searchSounds(String query) async {
    try {
      final r = await _dio.get<Map<String, dynamic>>('/sounds/search', queryParameters: {'q': query});
      final list = r.data?['sounds'] as List? ?? [];
      return list.map((e) => SoundModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw ServerException(message: e.message ?? 'Failed to search sounds');
    }
  }
}
