import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/sounds/data/datasources/sound_remote_datasource.dart';
import 'package:tiktok_clone/features/sounds/domain/entities/sound_entity.dart';

final soundDatasourceProvider = Provider<SoundRemoteDatasource>((ref) =>
    SoundRemoteDatasourceImpl(ApiClient.instance.dio));

final soundProvider = FutureProvider.family<SoundEntity, String>((ref, soundId) async {
  return ref.watch(soundDatasourceProvider).getSound(soundId);
});

final trendingSoundsProvider = FutureProvider<List<SoundEntity>>((ref) async {
  return ref.watch(soundDatasourceProvider).getTrendingSounds();
});
