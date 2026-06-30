class SoundEntity {
  const SoundEntity({
    required this.id,
    required this.title,
    required this.artistName,
    required this.coverUrl,
    required this.audioUrl,
    required this.duration,
    required this.usageCount,
    this.isOriginal = false,
    this.isTrending = false,
  });
  final String id;
  final String title;
  final String artistName;
  final String coverUrl;
  final String audioUrl;
  final int duration;
  final int usageCount;
  final bool isOriginal;
  final bool isTrending;
}
