import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/order_entity.dart';
import '../entities/product_entity.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Paginated result wrapper
// ─────────────────────────────────────────────────────────────────────────────

class PaginatedProducts {
  const PaginatedProducts({required this.items, this.nextCursor});
  final List<ProductEntity> items;
  final String? nextCursor;
}

class PaginatedOrders {
  const PaginatedOrders({required this.items, this.nextCursor});
  final List<OrderEntity> items;
  final String? nextCursor;
}

// ─────────────────────────────────────────────────────────────────────────────
// EcommerceRepository
// ─────────────────────────────────────────────────────────────────────────────

abstract class EcommerceRepository {
  // ── Products ──────────────────────────────────────────────────────────────
  Future<Either<Failure, PaginatedProducts>> getProducts({
    String? category,
    String? cursor,
  });

  Future<Either<Failure, ProductEntity>> getProduct(String id);

  Future<Either<Failure, PaginatedProducts>> searchProducts({
    required String query,
    String? cursor,
  });

  // ── Cart ──────────────────────────────────────────────────────────────────
  Future<Either<Failure, CartEntity>> getCart();

  Future<Either<Failure, CartEntity>> addToCart({
    required String productId,
    required String? variantId,
    required int qty,
  });

  Future<Either<Failure, CartEntity>> updateCartItem({
    required String itemId,
    required int qty,
  });

  Future<Either<Failure, CartEntity>> removeCartItem(String itemId);

  // ── Orders ────────────────────────────────────────────────────────────────
  Future<Either<Failure, OrderEntity>> placeOrder({
    required BuyerInfoEntity shippingAddress,
    required String paymentMethod,
  });

  Future<Either<Failure, PaginatedOrders>> getOrders({String? cursor});

  Future<Either<Failure, OrderEntity>> getOrder(String id);

  Future<Either<Failure, OrderEntity>> cancelOrder(String id);
}
