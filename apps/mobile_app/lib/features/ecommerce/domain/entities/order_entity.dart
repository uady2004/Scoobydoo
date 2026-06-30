import 'package:equatable/equatable.dart';
import 'product_entity.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Enums
// ─────────────────────────────────────────────────────────────────────────────

enum OrderStatus {
  pending,
  processing,
  shipped,
  delivered,
  cancelled,
  returned;

  String get label {
    switch (this) {
      case OrderStatus.pending:
        return 'Pending';
      case OrderStatus.processing:
        return 'Processing';
      case OrderStatus.shipped:
        return 'Shipped';
      case OrderStatus.delivered:
        return 'Delivered';
      case OrderStatus.cancelled:
        return 'Cancelled';
      case OrderStatus.returned:
        return 'Returned';
    }
  }

  static OrderStatus fromString(String value) {
    return OrderStatus.values.firstWhere(
      (e) => e.name == value,
      orElse: () => OrderStatus.pending,
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// BuyerInfoEntity
// ─────────────────────────────────────────────────────────────────────────────

class BuyerInfoEntity extends Equatable {
  const BuyerInfoEntity({
    required this.name,
    required this.phone,
    required this.addressLine1,
    this.addressLine2,
    required this.city,
    required this.state,
    required this.postalCode,
    required this.country,
  });

  final String name;
  final String phone;
  final String addressLine1;
  final String? addressLine2;
  final String city;
  final String state;
  final String postalCode;
  final String country;

  String get fullAddress {
    final parts = [
      addressLine1,
      if (addressLine2 != null && addressLine2!.isNotEmpty) addressLine2,
      city,
      state,
      postalCode,
      country,
    ];
    return parts.join(', ');
  }

  @override
  List<Object?> get props =>
      [name, phone, addressLine1, addressLine2, city, state, postalCode, country];
}

// ─────────────────────────────────────────────────────────────────────────────
// OrderItemEntity
// ─────────────────────────────────────────────────────────────────────────────

class OrderItemEntity extends Equatable {
  const OrderItemEntity({
    required this.product,
    required this.variant,
    required this.qty,
    required this.unitPrice,
  });

  final ProductEntity product;
  final ProductVariantEntity? variant;
  final int qty;
  final double unitPrice;

  double get subtotal => unitPrice * qty;

  @override
  List<Object?> get props => [product, variant, qty, unitPrice];
}

// ─────────────────────────────────────────────────────────────────────────────
// OrderEntity
// ─────────────────────────────────────────────────────────────────────────────

class OrderEntity extends Equatable {
  const OrderEntity({
    required this.id,
    required this.buyerInfo,
    required this.items,
    required this.status,
    required this.subtotal,
    required this.shippingFee,
    required this.total,
    this.trackingNumber,
    this.courierName,
    required this.paymentMethod,
    required this.createdAt,
  });

  final String id;
  final BuyerInfoEntity buyerInfo;
  final List<OrderItemEntity> items;
  final OrderStatus status;
  final double subtotal;
  final double shippingFee;
  final double total;
  final String? trackingNumber;
  final String? courierName;
  final String paymentMethod;
  final DateTime createdAt;

  int get itemCount => items.fold(0, (sum, item) => sum + item.qty);

  List<String> get thumbnailUrls =>
      items.map((i) => i.product.thumbnailUrl).where((u) => u.isNotEmpty).toList();

  bool get isActive =>
      status == OrderStatus.pending ||
      status == OrderStatus.processing ||
      status == OrderStatus.shipped;

  @override
  List<Object?> get props => [id, status, total, createdAt];
}

// ─────────────────────────────────────────────────────────────────────────────
// CartItemEntity
// ─────────────────────────────────────────────────────────────────────────────

class CartItemEntity extends Equatable {
  const CartItemEntity({
    required this.id,
    required this.productId,
    required this.variantId,
    required this.qty,
    required this.product,
  });

  final String id;
  final String productId;
  final String? variantId;
  final int qty;
  final ProductEntity product;

  ProductVariantEntity? get selectedVariant {
    if (variantId == null) return null;
    try {
      return product.variants.firstWhere((v) => v.id == variantId);
    } catch (_) {
      return null;
    }
  }

  double get unitPrice => selectedVariant?.price ?? product.effectivePrice;
  double get subtotal => unitPrice * qty;

  @override
  List<Object?> get props => [id, productId, variantId, qty];
}

// ─────────────────────────────────────────────────────────────────────────────
// CartEntity
// ─────────────────────────────────────────────────────────────────────────────

class CartEntity extends Equatable {
  const CartEntity({
    required this.items,
  });

  const CartEntity.empty() : items = const [];

  final List<CartItemEntity> items;

  double get subtotal =>
      items.fold(0.0, (sum, item) => sum + item.subtotal);

  double get shippingFee => items.isEmpty ? 0.0 : 2.99;

  double get total => subtotal + shippingFee;

  int get itemCount => items.fold(0, (sum, item) => sum + item.qty);

  bool get isEmpty => items.isEmpty;

  @override
  List<Object?> get props => [items];
}
