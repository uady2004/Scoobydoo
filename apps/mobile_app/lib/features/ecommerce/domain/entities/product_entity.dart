import 'package:equatable/equatable.dart';

// ─────────────────────────────────────────────────────────────────────────────
// ProductVariantEntity
// ─────────────────────────────────────────────────────────────────────────────

class ProductVariantEntity extends Equatable {
  const ProductVariantEntity({
    required this.id,
    required this.name,
    required this.sku,
    required this.price,
    required this.stockQty,
    required this.attributes,
  });

  final String id;
  final String name;
  final String sku;
  final double price;
  final int stockQty;
  final Map<String, String> attributes;

  bool get isAvailable => stockQty > 0;

  @override
  List<Object?> get props => [id, name, sku, price, stockQty, attributes];
}

// ─────────────────────────────────────────────────────────────────────────────
// ProductEntity
// ─────────────────────────────────────────────────────────────────────────────

class ProductEntity extends Equatable {
  const ProductEntity({
    required this.id,
    required this.sellerId,
    required this.sellerName,
    required this.sellerAvatarUrl,
    required this.name,
    required this.description,
    required this.price,
    this.discountPrice,
    required this.images,
    required this.variants,
    required this.category,
    required this.rating,
    required this.reviewCount,
    required this.soldCount,
    required this.inStock,
    required this.tags,
  });

  final String id;
  final String sellerId;
  final String sellerName;
  final String sellerAvatarUrl;
  final String name;
  final String description;
  final double price;
  final double? discountPrice;
  final List<String> images;
  final List<ProductVariantEntity> variants;
  final String category;
  final double rating;
  final int reviewCount;
  final int soldCount;
  final bool inStock;
  final List<String> tags;

  double get effectivePrice => discountPrice ?? price;

  double? get discountPercentage {
    if (discountPrice == null || discountPrice! >= price) return null;
    return ((price - discountPrice!) / price * 100);
  }

  String get thumbnailUrl => images.isNotEmpty ? images.first : '';

  @override
  List<Object?> get props => [
        id,
        sellerId,
        name,
        price,
        discountPrice,
        category,
        rating,
        inStock,
      ];
}
