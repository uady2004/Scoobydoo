import 'package:tiktok_clone/features/gifts/domain/entities/gift_entity.dart';

// ─────────────────────────────────────────────────────────────────────────────
// GiftModel
// ─────────────────────────────────────────────────────────────────────────────

class GiftModel extends GiftEntity {
  const GiftModel({
    required super.id,
    required super.name,
    required super.coinCost,
    required super.animationKey,
    required super.previewImageUrl,
    required super.category,
  });

  factory GiftModel.fromJson(Map<String, dynamic> json) => GiftModel(
        id: json['id'] as String,
        name: json['name'] as String,
        coinCost: (json['coin_cost'] as num).toInt(),
        animationKey: json['animation_key'] as String? ?? '',
        previewImageUrl: json['preview_image_url'] as String? ?? '',
        category: json['category'] as String? ?? 'basic',
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'coin_cost': coinCost,
        'animation_key': animationKey,
        'preview_image_url': previewImageUrl,
        'category': category,
      };

  /// Default catalog used when the backend is unavailable.
  static List<GiftModel> get defaults => const [
        GiftModel(
          id: 'g1',
          name: 'Rose',
          coinCost: 1,
          animationKey: 'rose',
          previewImageUrl: '',
          category: 'basic',
        ),
        GiftModel(
          id: 'g2',
          name: 'TikTok',
          coinCost: 1,
          animationKey: 'tiktok',
          previewImageUrl: '',
          category: 'basic',
        ),
        GiftModel(
          id: 'g3',
          name: 'Heart',
          coinCost: 5,
          animationKey: 'heart',
          previewImageUrl: '',
          category: 'basic',
        ),
        GiftModel(
          id: 'g4',
          name: 'Sunglasses',
          coinCost: 5,
          animationKey: 'sunglasses',
          previewImageUrl: '',
          category: 'basic',
        ),
        GiftModel(
          id: 'g5',
          name: 'Perfume',
          coinCost: 20,
          animationKey: 'perfume',
          previewImageUrl: '',
          category: 'premium',
        ),
        GiftModel(
          id: 'g6',
          name: 'Mic',
          coinCost: 20,
          animationKey: 'mic',
          previewImageUrl: '',
          category: 'premium',
        ),
        GiftModel(
          id: 'g7',
          name: 'Sports Car',
          coinCost: 100,
          animationKey: 'car',
          previewImageUrl: '',
          category: 'luxury',
        ),
        GiftModel(
          id: 'g8',
          name: 'Universe',
          coinCost: 1000,
          animationKey: 'universe',
          previewImageUrl: '',
          category: 'luxury',
        ),
      ];

}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy alias — kept for backward compatibility with livestream widgets
// that reference kDefaultGifts.
// ─────────────────────────────────────────────────────────────────────────────

final kDefaultGifts = GiftModel.defaults;
