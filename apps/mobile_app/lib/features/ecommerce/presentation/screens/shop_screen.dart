import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/entities/product_entity.dart';
import '../providers/ecommerce_provider.dart';
import 'product_screen.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const _categories = [
  'All',
  'Clothing',
  'Electronics',
  'Beauty',
  'Food',
  'Home',
  'Sports',
];

// ─────────────────────────────────────────────────────────────────────────────
// ShopScreen
// ─────────────────────────────────────────────────────────────────────────────

class ShopScreen extends ConsumerStatefulWidget {
  const ShopScreen({super.key});

  @override
  ConsumerState<ShopScreen> createState() => _ShopScreenState();
}

class _ShopScreenState extends ConsumerState<ShopScreen> {
  final _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 400) {
      ref.read(productsProvider.notifier).loadMore();
    }
  }

  void _openSearch() {
    Navigator.of(context).pushNamed('/search', arguments: {'type': 'products'});
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: NestedScrollView(
          controller: _scrollController,
          headerSliverBuilder: (context, innerBoxIsScrolled) => [
            _SearchAppBar(onSearchTap: _openSearch),
            _CategoryBar(),
          ],
          body: _ProductGrid(),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Search AppBar
// ─────────────────────────────────────────────────────────────────────────────

class _SearchAppBar extends StatelessWidget {
  const _SearchAppBar({required this.onSearchTap});
  final VoidCallback onSearchTap;

  @override
  Widget build(BuildContext context) {
    return SliverAppBar(
      backgroundColor: Colors.black,
      floating: true,
      snap: true,
      elevation: 0,
      title: GestureDetector(
        onTap: onSearchTap,
        child: Container(
          height: 40,
          decoration: BoxDecoration(
            color: const Color(0xFF1A1A1A),
            borderRadius: BorderRadius.circular(20),
          ),
          child: const Row(
            children: [
              SizedBox(width: 12),
              Icon(Icons.search, color: Color(0xFF888888), size: 20),
              SizedBox(width: 8),
              Text(
                'Search products...',
                style: TextStyle(
                  color: Color(0xFF888888),
                  fontSize: 14,
                  fontWeight: FontWeight.w400,
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        IconButton(
          icon: const Icon(Icons.shopping_bag_outlined, color: Colors.white),
          onPressed: () => Navigator.of(context).pushNamed('/cart'),
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Category horizontal bar
// ─────────────────────────────────────────────────────────────────────────────

class _CategoryBar extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(productsProvider).valueOrNull?.selectedCategory
        ?? 'All';

    return SliverToBoxAdapter(
      child: SizedBox(
        height: 44,
        child: ListView.separated(
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.symmetric(horizontal: 12),
          itemCount: _categories.length,
          separatorBuilder: (_, __) => const SizedBox(width: 8),
          itemBuilder: (context, index) {
            final cat = _categories[index];
            final isSelected = cat == selected;
            return GestureDetector(
              onTap: () {
                ref.read(productsProvider.notifier).selectCategory(cat);
              },
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                padding: const EdgeInsets.symmetric(
                    horizontal: 16, vertical: 8),
                decoration: BoxDecoration(
                  color: isSelected
                      ? const Color(0xFFFF2D55)
                      : const Color(0xFF1A1A1A),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(
                  cat,
                  style: TextStyle(
                    color: isSelected ? Colors.white : const Color(0xFFAAAAAA),
                    fontSize: 13,
                    fontWeight: isSelected
                        ? FontWeight.w600
                        : FontWeight.w400,
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Product grid
// ─────────────────────────────────────────────────────────────────────────────

class _ProductGrid extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(productsProvider);

    return asyncState.when(
      loading: () => const Center(
        child: CircularProgressIndicator(
          color: Color(0xFFFF2D55),
          strokeWidth: 2,
        ),
      ),
      error: (err, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, color: Color(0xFFFF2D55), size: 48),
            const SizedBox(height: 12),
            Text(
              err.toString(),
              style: const TextStyle(color: Colors.white70, fontSize: 14),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            TextButton(
              onPressed: () =>
                  ref.refresh(productsProvider),
              child: const Text(
                'Try again',
                style: TextStyle(color: Color(0xFFFF2D55)),
              ),
            ),
          ],
        ),
      ),
      data: (state) {
        if (state.items.isEmpty) {
          return const _EmptyProducts();
        }
        return CustomScrollView(
          slivers: [
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
              sliver: SliverGrid(
                delegate: SliverChildBuilderDelegate(
                  (context, index) {
                    if (index >= state.items.length) {
                      return const _LoadingTile();
                    }
                    return _ProductCard(product: state.items[index]);
                  },
                  childCount: state.isLoadingMore
                      ? state.items.length + 2
                      : state.items.length,
                ),
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 2,
                  mainAxisSpacing: 10,
                  crossAxisSpacing: 10,
                  childAspectRatio: 0.72,
                ),
              ),
            ),
            if (!state.isLoadingMore && state.hasReachedEnd && state.items.isNotEmpty)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.all(24),
                  child: Center(
                    child: Text(
                      "You've seen everything",
                      style: TextStyle(color: Color(0xFF555555), fontSize: 12),
                    ),
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}

class _EmptyProducts extends StatelessWidget {
  const _EmptyProducts();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.storefront_outlined, color: Color(0xFF444444), size: 64),
          SizedBox(height: 16),
          Text(
            'No products found',
            style: TextStyle(color: Colors.white70, fontSize: 16),
          ),
        ],
      ),
    );
  }
}

class _LoadingTile extends StatelessWidget {
  const _LoadingTile();

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: const Color(0xFF1A1A1A),
        borderRadius: BorderRadius.circular(12),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// ProductCard
// ─────────────────────────────────────────────────────────────────────────────

class _ProductCard extends ConsumerWidget {
  const _ProductCard({required this.product});
  final ProductEntity product;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return GestureDetector(
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => ProductScreen(productId: product.id),
        ),
      ),
      child: Container(
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
        ),
        clipBehavior: Clip.hardEdge,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Thumbnail ──────────────────────────────────────────────────
            Expanded(
              flex: 60,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  _ProductImage(imageUrl: product.thumbnailUrl),
                  if (!product.inStock)
                    Container(
                      color: Colors.black54,
                      child: const Center(
                        child: Text(
                          'Out of Stock',
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ),
                  if (product.discountPercentage != null)
                    Positioned(
                      top: 8,
                      left: 8,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFF2D55),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '-${product.discountPercentage!.toStringAsFixed(0)}%',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 10,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ),
            // ── Info ───────────────────────────────────────────────────────
            Expanded(
              flex: 40,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(8, 8, 8, 6),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      product.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Text(
                          '\$${product.effectivePrice.toStringAsFixed(2)}',
                          style: const TextStyle(
                            color: Color(0xFFFF2D55),
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        if (product.discountPrice != null) ...[
                          const SizedBox(width: 4),
                          Text(
                            '\$${product.price.toStringAsFixed(2)}',
                            style: const TextStyle(
                              color: Color(0xFF666666),
                              fontSize: 11,
                              decoration: TextDecoration.lineThrough,
                              decorationColor: Color(0xFF666666),
                            ),
                          ),
                        ],
                      ],
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        const Icon(Icons.star,
                            color: Color(0xFFFFC107), size: 12),
                        const SizedBox(width: 2),
                        Text(
                          product.rating.toStringAsFixed(1),
                          style: const TextStyle(
                            color: Color(0xFFAAAAAA),
                            fontSize: 11,
                          ),
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            '${_formatCount(product.soldCount)} sold',
                            style: const TextStyle(
                              color: Color(0xFF666666),
                              fontSize: 11,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    const Spacer(),
                    SizedBox(
                      width: double.infinity,
                      height: 28,
                      child: product.inStock
                          ? _AddToCartButton(product: product)
                          : Container(
                              alignment: Alignment.center,
                              decoration: BoxDecoration(
                                color: const Color(0xFF2A2A2A),
                                borderRadius: BorderRadius.circular(6),
                              ),
                              child: const Text(
                                'Sold Out',
                                style: TextStyle(
                                  color: Color(0xFF666666),
                                  fontSize: 11,
                                ),
                              ),
                            ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AddToCartButton extends ConsumerStatefulWidget {
  const _AddToCartButton({required this.product});
  final ProductEntity product;

  @override
  ConsumerState<_AddToCartButton> createState() => _AddToCartButtonState();
}

class _AddToCartButtonState extends ConsumerState<_AddToCartButton> {
  bool _loading = false;

  Future<void> _add() async {
    if (_loading) return;
    setState(() => _loading = true);
    await ref.read(cartProvider.notifier).addItem(
          productId: widget.product.id,
          variantId: widget.product.variants.isNotEmpty
              ? widget.product.variants.first.id
              : null,
        );
    if (mounted) setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    return TextButton(
      onPressed: _loading ? null : _add,
      style: TextButton.styleFrom(
        backgroundColor: const Color(0xFF1A1A1A),
        foregroundColor: const Color(0xFFFF2D55),
        padding: EdgeInsets.zero,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(6),
          side: const BorderSide(color: Color(0xFFFF2D55), width: 0.5),
        ),
        minimumSize: Size.zero,
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
      ),
      child: _loading
          ? const SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                strokeWidth: 1.5,
                color: Color(0xFFFF2D55),
              ),
            )
          : const Text(
              'Add to Cart',
              style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600),
            ),
    );
  }
}

class _ProductImage extends StatelessWidget {
  const _ProductImage({required this.imageUrl});
  final String imageUrl;

  @override
  Widget build(BuildContext context) {
    if (imageUrl.isEmpty) {
      return Container(
        color: const Color(0xFF1A1A1A),
        child: const Icon(Icons.image_not_supported_outlined,
            color: Color(0xFF333333), size: 40),
      );
    }
    return Image.network(
      imageUrl,
      fit: BoxFit.cover,
      errorBuilder: (_, __, ___) => Container(
        color: const Color(0xFF1A1A1A),
        child: const Icon(Icons.broken_image_outlined,
            color: Color(0xFF333333), size: 40),
      ),
      loadingBuilder: (_, child, progress) {
        if (progress == null) return child;
        return Container(color: const Color(0xFF1A1A1A));
      },
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

String _formatCount(int count) {
  if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
  if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
  return count.toString();
}
