import '../../../sounds/domain/entities/sound_entity.dart';

class SoundModel extends SoundEntity {
  const SoundModel({
    required super.id,
    required super.title,
    required super.artistName,
    required super.coverUrl,
    required super.audioUrl,
    required super.duration,
    required super.usageCount,
    super.isOriginal,
    super.isTrending,
  });

  factory SoundModel.fromJson(Map<String, dynamic> j) => SoundModel(
    id: j['id'] as String,
    title: j['title'] as String? ?? '',
    artistName: j['artist_name'] as String? ?? '',
    coverUrl: j['cover_url'] as String? ?? '',
    audioUrl: j['audio_url'] as String? ?? '',
    duration: (j['duration'] as num? ?? 0).toInt(),
    usageCount: (j['usage_count'] as num? ?? 0).toInt(),
    isOriginal: j['is_original'] as bool? ?? false,
    isTrending: j['is_trending'] as bool? ?? false,
  );
}
