import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/datasources/ecommerce_remote_datasource.dart';
import '../../data/repositories/ecommerce_repository_impl.dart';
import '../../domain/entities/order_entity.dart';
import '../../domain/entities/product_entity.dart';
import '../../domain/repositories/ecommerce_repository.dart';
import '../../domain/usecases/add_to_cart_usecase.dart';
import '../../domain/usecases/get_products_usecase.dart';
import '../../domain/usecases/place_order_usecase.dart';
import '../../../../core/usecases/usecase.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Dependency providers
// ─────────────────────────────────────────────────────────────────────────────

final ecommerceRemoteDataSourceProvider =
    Provider<EcommerceRemoteDataSource>((ref) {
  return EcommerceRemoteDataSourceImpl();
});

final ecommerceRepositoryProvider =
    Provider<EcommerceRepository>((ref) {
  return EcommerceRepositoryImpl(
    ref.read(ecommerceRemoteDataSourceProvider),
  );
});

final getProductsUseCaseProvider =
    Provider<GetProductsUseCase>((ref) {
  return GetProductsUseCase(ref.read(ecommerceRepositoryProvider));
});

final searchProductsUseCaseProvider =
    Provider<SearchProductsUseCase>((ref) {
  return SearchProductsUseCase(ref.read(ecommerceRepositoryProvider));
});

final addToCartUseCaseProvider = Provider<AddToCartUseCase>((ref) {
  return AddToCartUseCase(ref.read(ecommerceRepositoryProvider));
});

final updateCartItemUseCaseProvider =
    Provider<UpdateCartItemUseCase>((ref) {
  return UpdateCartItemUseCase(ref.read(ecommerceRepositoryProvider));
});

final removeCartItemUseCaseProvider =
    Provider<RemoveCartItemUseCase>((ref) {
  return RemoveCartItemUseCase(ref.read(ecommerceRepositoryProvider));
});

final getCartUseCaseProvider = Provider<GetCartUseCase>((ref) {
  return GetCartUseCase(ref.read(ecommerceRepositoryProvider));
});

final placeOrderUseCaseProvider = Provider<PlaceOrderUseCase>((ref) {
  return PlaceOrderUseCase(ref.read(ecommerceRepositoryProvider));
});

final getOrdersUseCaseProvider = Provider<GetOrdersUseCase>((ref) {
  return GetOrdersUseCase(ref.read(ecommerceRepositoryProvider));
});

final cancelOrderUseCaseProvider = Provider<CancelOrderUseCase>((ref) {
  return CancelOrderUseCase(ref.read(ecommerceRepositoryProvider));
});

// ─────────────────────────────────────────────────────────────────────────────
// Products state + notifier
// ─────────────────────────────────────────────────────────────────────────────

class ProductsState {
  const ProductsState({
    this.items = const [],
    this.nextCursor,
    this.isLoadingMore = false,
    this.hasReachedEnd = false,
    this.selectedCategory = 'All',
    this.error,
  });

  final List<ProductEntity> items;
  final String? nextCursor;
  final bool isLoadingMore;
  final bool hasReachedEnd;
  final String selectedCategory;
  final String? error;

  ProductsState copyWith({
    List<ProductEntity>? items,
    String? nextCursor,
    bool? isLoadingMore,
    bool? hasReachedEnd,
    String? selectedCategory,
    String? error,
    bool clearError = false,
    bool clearCursor = false,
  }) {
    return ProductsState(
      items: items ?? this.items,
      nextCursor: clearCursor ? null : (nextCursor ?? this.nextCursor),
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasReachedEnd: hasReachedEnd ?? this.hasReachedEnd,
      selectedCategory: selectedCategory ?? this.selectedCategory,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

class ProductsNotifier extends AsyncNotifier<ProductsState> {
  @override
  Future<ProductsState> build() async {
    return _fetchPage(category: 'All', cursor: null, existing: []);
  }

  Future<void> selectCategory(String category) async {
    state = AsyncData(
      state.valueOrNull?.copyWith(
            selectedCategory: category,
            items: [],
            clearCursor: true,
            hasReachedEnd: false,
            clearError: true,
          ) ??
          ProductsState(selectedCategory: category),
    );
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _fetchPage(category: category, cursor: null, existing: []),
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || current.isLoadingMore || current.hasReachedEnd) {
      return;
    }
    state = AsyncData(current.copyWith(isLoadingMore: true, clearError: true));

    final useCase = ref.read(getProductsUseCaseProvider);
    final result = await useCase(GetProductsParams(
      category: current.selectedCategory,
      cursor: current.nextCursor,
    ));

    result.fold(
      (failure) => state = AsyncData(
        current.copyWith(isLoadingMore: false, error: failure.message),
      ),
      (page) => state = AsyncData(current.copyWith(
        items: [...current.items, ...page.items],
        nextCursor: page.nextCursor,
        isLoadingMore: false,
        hasReachedEnd: page.nextCursor == null,
        clearError: true,
      )),
    );
  }

  Future<ProductsState> _fetchPage({
    required String category,
    required String? cursor,
    required List<ProductEntity> existing,
  }) async {
    final useCase = ref.read(getProductsUseCaseProvider);
    final result = await useCase(GetProductsParams(
      category: category == 'All' ? null : category,
      cursor: cursor,
    ));
    return result.fold(
      (failure) => throw Exception(failure.message),
      (page) => ProductsState(
        items: [...existing, ...page.items],
        nextCursor: page.nextCursor,
        hasReachedEnd: page.nextCursor == null,
        selectedCategory: category,
      ),
    );
  }
}

final productsProvider =
    AsyncNotifierProvider<ProductsNotifier, ProductsState>(
  ProductsNotifier.new,
);

// ─────────────────────────────────────────────────────────────────────────────
// Single product provider (family)
// ─────────────────────────────────────────────────────────────────────────────

final productDetailProvider =
    FutureProvider.family<ProductEntity, String>((ref, id) async {
  final repo = ref.read(ecommerceRepositoryProvider);
  final result = await repo.getProduct(id);
  return result.fold(
    (failure) => throw Exception(failure.message),
    (product) => product,
  );
});

// ─────────────────────────────────────────────────────────────────────────────
// Cart notifier
// ─────────────────────────────────────────────────────────────────────────────

class CartNotifier extends AsyncNotifier<CartEntity> {
  @override
  Future<CartEntity> build() async {
    final useCase = ref.read(getCartUseCaseProvider);
    final result = await useCase(const NoParams());
    return result.fold(
      (failure) => const CartEntity.empty(),
      (cart) => cart,
    );
  }

  Future<void> addItem({
    required String productId,
    String? variantId,
    int qty = 1,
  }) async {
    final current = state.valueOrNull ?? const CartEntity.empty();
    // Optimistic: find existing item and increment locally
    final useCase = ref.read(addToCartUseCaseProvider);
    final result = await useCase(AddToCartParams(
      productId: productId,
      variantId: variantId,
      qty: qty,
    ));
    result.fold(
      (failure) => state = AsyncData(current), // revert
      (cart) => state = AsyncData(cart),
    );
  }

  Future<void> updateQty({required String itemId, required int qty}) async {
    final current = state.valueOrNull ?? const CartEntity.empty();

    if (qty <= 0) {
      await removeItem(itemId);
      return;
    }

    // Optimistic update
    final optimistic = CartEntity(
      items: current.items
          .map((i) => i.id == itemId
              ? CartItemEntity(
                  id: i.id,
                  productId: i.productId,
                  variantId: i.variantId,
                  qty: qty,
                  product: i.product,
                )
              : i)
          .toList(),
    );
    state = AsyncData(optimistic);

    final useCase = ref.read(updateCartItemUseCaseProvider);
    final result = await useCase(
        UpdateCartItemParams(itemId: itemId, qty: qty));
    result.fold(
      (failure) => state = AsyncData(current), // revert
      (cart) => state = AsyncData(cart),
    );
  }

  Future<void> removeItem(String itemId) async {
    final current = state.valueOrNull ?? const CartEntity.empty();

    // Optimistic remove
    state = AsyncData(CartEntity(
      items: current.items.where((i) => i.id != itemId).toList(),
    ));

    final useCase = ref.read(removeCartItemUseCaseProvider);
    final result = await useCase(itemId);
    result.fold(
      (failure) => state = AsyncData(current), // revert
      (cart) => state = AsyncData(cart),
    );
  }

  void clear() {
    state = const AsyncData(CartEntity.empty());
  }
}

final cartProvider =
    AsyncNotifierProvider<CartNotifier, CartEntity>(CartNotifier.new);

// ─────────────────────────────────────────────────────────────────────────────
// Orders state + notifier
// ─────────────────────────────────────────────────────────────────────────────

class OrdersState {
  const OrdersState({
    this.items = const [],
    this.nextCursor,
    this.isLoadingMore = false,
    this.hasReachedEnd = false,
    this.error,
  });

  final List<OrderEntity> items;
  final String? nextCursor;
  final bool isLoadingMore;
  final bool hasReachedEnd;
  final String? error;

  OrdersState copyWith({
    List<OrderEntity>? items,
    String? nextCursor,
    bool? isLoadingMore,
    bool? hasReachedEnd,
    String? error,
    bool clearError = false,
  }) {
    return OrdersState(
      items: items ?? this.items,
      nextCursor: nextCursor ?? this.nextCursor,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasReachedEnd: hasReachedEnd ?? this.hasReachedEnd,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

class OrdersNotifier extends AsyncNotifier<OrdersState> {
  @override
  Future<OrdersState> build() async {
    return _fetchPage(cursor: null, existing: []);
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _fetchPage(cursor: null, existing: []),
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || current.isLoadingMore || current.hasReachedEnd) {
      return;
    }
    state = AsyncData(current.copyWith(isLoadingMore: true, clearError: true));

    final useCase = ref.read(getOrdersUseCaseProvider);
    final result =
        await useCase(GetOrdersParams(cursor: current.nextCursor));

    result.fold(
      (failure) => state = AsyncData(
        current.copyWith(isLoadingMore: false, error: failure.message),
      ),
      (page) => state = AsyncData(current.copyWith(
        items: [...current.items, ...page.items],
        nextCursor: page.nextCursor,
        isLoadingMore: false,
        hasReachedEnd: page.nextCursor == null,
        clearError: true,
      )),
    );
  }

  void prependOrder(OrderEntity order) {
    final current = state.valueOrNull;
    if (current == null) return;
    state = AsyncData(current.copyWith(items: [order, ...current.items]));
  }

  Future<OrdersState> _fetchPage({
    required String? cursor,
    required List<OrderEntity> existing,
  }) async {
    final useCase = ref.read(getOrdersUseCaseProvider);
    final result = await useCase(GetOrdersParams(cursor: cursor));
    return result.fold(
      (failure) => throw Exception(failure.message),
      (page) => OrdersState(
        items: [...existing, ...page.items],
        nextCursor: page.nextCursor,
        hasReachedEnd: page.nextCursor == null,
      ),
    );
  }
}

final ordersProvider =
    AsyncNotifierProvider<OrdersNotifier, OrdersState>(OrdersNotifier.new);

// ─────────────────────────────────────────────────────────────────────────────
// Single order provider (family)
// ─────────────────────────────────────────────────────────────────────────────

final orderDetailProvider =
    FutureProvider.family<OrderEntity, String>((ref, id) async {
  final repo = ref.read(ecommerceRepositoryProvider);
  final result = await repo.getOrder(id);
  return result.fold(
    (failure) => throw Exception(failure.message),
    (order) => order,
  );
});
