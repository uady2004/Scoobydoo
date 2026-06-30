import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/entities/product_entity.dart';
import '../providers/ecommerce_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// ProductScreen
// ─────────────────────────────────────────────────────────────────────────────

class ProductScreen extends ConsumerStatefulWidget {
  const ProductScreen({super.key, required this.productId});
  final String productId;

  @override
  ConsumerState<ProductScreen> createState() => _ProductScreenState();
}

class _ProductScreenState extends ConsumerState<ProductScreen> {
  int _currentImageIndex = 0;
  String? _selectedVariantId;
  int _qty = 1;
  bool _descriptionExpanded = false;
  bool _addingToCart = false;

  @override
  Widget build(BuildContext context) {
    final asyncProduct = ref.watch(productDetailProvider(widget.productId));

    return Scaffold(
      backgroundColor: Colors.black,
      body: asyncProduct.when(
        loading: () => const Center(
          child: CircularProgressIndicator(
              color: Color(0xFFFF2D55), strokeWidth: 2),
        ),
        error: (err, _) => Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(Icons.error_outline,
                  color: Color(0xFFFF2D55), size: 48),
              const SizedBox(height: 12),
              Text(err.toString(),
                  style: const TextStyle(color: Colors.white70)),
              TextButton(
                onPressed: () =>
                    ref.invalidate(productDetailProvider(widget.productId)),
                child: const Text('Retry',
                    style: TextStyle(color: Color(0xFFFF2D55))),
              ),
            ],
          ),
        ),
        data: (product) {
          // Auto-select first variant
          if (_selectedVariantId == null && product.variants.isNotEmpty) {
            _selectedVariantId = product.variants.first.id;
          }
          return Stack(
            children: [
              CustomScrollView(
                slivers: [
                  _ImageSliver(
                    product: product,
                    currentIndex: _currentImageIndex,
                    onPageChanged: (i) =>
                        setState(() => _currentImageIndex = i),
                  ),
                  SliverToBoxAdapter(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _ProductInfo(product: product),
                        const _Divider(),
                        if (product.variants.isNotEmpty)
                          _VariantsSection(
                            product: product,
                            selectedVariantId: _selectedVariantId,
                            onSelect: (id) =>
                                setState(() => _selectedVariantId = id),
                          ),
                        if (product.variants.isNotEmpty) const _Divider(),
                        _SellerRow(product: product),
                        const _Divider(),
                        _DescriptionSection(
                          description: product.description,
                          expanded: _descriptionExpanded,
                          onToggle: () => setState(
                              () => _descriptionExpanded = !_descriptionExpanded),
                        ),
                        const _Divider(),
                        _ReviewsSection(product: product),
                        // Spacer so sticky bar doesn't overlap content
                        const SizedBox(height: 88),
                      ],
                    ),
                  ),
                ],
              ),
              // ── Sticky bottom bar ──────────────────────────────────────
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                child: _StickyBottomBar(
                  product: product,
                  qty: _qty,
                  onQtyChange: (q) => setState(() => _qty = q),
                  selectedVariantId: _selectedVariantId,
                  addingToCart: _addingToCart,
                  onAddToCart: () => _addToCart(product),
                  onBuyNow: () => _buyNow(product),
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _addToCart(ProductEntity product) async {
    if (_addingToCart) return;
    setState(() => _addingToCart = true);
    await ref.read(cartProvider.notifier).addItem(
          productId: product.id,
          variantId: _selectedVariantId,
          qty: _qty,
        );
    if (!mounted) return;
    setState(() => _addingToCart = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        backgroundColor: const Color(0xFF1A1A1A),
        content: Row(
          children: [
            const Icon(Icons.check_circle,
                color: Color(0xFFFF2D55), size: 18),
            const SizedBox(width: 8),
            const Text('Added to cart',
                style: TextStyle(color: Colors.white)),
            const Spacer(),
            TextButton(
              onPressed: () {
                ScaffoldMessenger.of(context).hideCurrentSnackBar();
                Navigator.of(context).pushNamed('/cart');
              },
              child: const Text('View',
                  style: TextStyle(color: Color(0xFFFF2D55))),
            ),
          ],
        ),
        duration: const Duration(seconds: 3),
      ),
    );
  }

  void _buyNow(ProductEntity product) {
    _addToCart(product).then((_) {
      if (mounted) Navigator.of(context).pushNamed('/cart');
    });
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Image sliver with PageView + dots
// ─────────────────────────────────────────────────────────────────────────────

class _ImageSliver extends StatelessWidget {
  const _ImageSliver({
    required this.product,
    required this.currentIndex,
    required this.onPageChanged,
  });

  final ProductEntity product;
  final int currentIndex;
  final ValueChanged<int> onPageChanged;

  @override
  Widget build(BuildContext context) {
    final images = product.images.isEmpty ? [''] : product.images;
    return SliverAppBar(
      expandedHeight: 300,
      pinned: true,
      backgroundColor: Colors.black,
      leading: IconButton(
        icon: Container(
          padding: const EdgeInsets.all(6),
          decoration: const BoxDecoration(
            color: Colors.black54,
            shape: BoxShape.circle,
          ),
          child: const Icon(Icons.arrow_back, color: Colors.white, size: 20),
        ),
        onPressed: () => Navigator.of(context).pop(),
      ),
      flexibleSpace: FlexibleSpaceBar(
        background: Stack(
          children: [
            PageView.builder(
              itemCount: images.length,
              onPageChanged: onPageChanged,
              itemBuilder: (context, index) {
                final url = images[index];
                if (url.isEmpty) {
                  return Container(
                    color: const Color(0xFF1A1A1A),
                    child: const Icon(Icons.image_not_supported_outlined,
                        color: Color(0xFF333333), size: 64),
                  );
                }
                return Image.network(
                  url,
                  fit: BoxFit.cover,
                  errorBuilder: (_, __, ___) => Container(
                    color: const Color(0xFF1A1A1A),
                    child: const Icon(Icons.broken_image_outlined,
                        color: Color(0xFF333333), size: 64),
                  ),
                  loadingBuilder: (_, child, progress) {
                    if (progress == null) return child;
                    return Container(color: const Color(0xFF1A1A1A));
                  },
                );
              },
            ),
            // Dot indicators
            if (images.length > 1)
              Positioned(
                bottom: 12,
                left: 0,
                right: 0,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: List.generate(images.length, (i) {
                    return AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      margin: const EdgeInsets.symmetric(horizontal: 3),
                      width: i == currentIndex ? 16 : 6,
                      height: 6,
                      decoration: BoxDecoration(
                        color: i == currentIndex
                            ? const Color(0xFFFF2D55)
                            : Colors.white38,
                        borderRadius: BorderRadius.circular(3),
                      ),
                    );
                  }),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Product info block
// ─────────────────────────────────────────────────────────────────────────────

class _ProductInfo extends StatelessWidget {
  const _ProductInfo({required this.product});
  final ProductEntity product;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            product.name,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 10),
          // Price row
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Text(
                '\$${product.effectivePrice.toStringAsFixed(2)}',
                style: const TextStyle(
                  color: Color(0xFFFF2D55),
                  fontSize: 24,
                  fontWeight: FontWeight.w800,
                ),
              ),
              if (product.discountPrice != null) ...[
                const SizedBox(width: 10),
                Text(
                  '\$${product.price.toStringAsFixed(2)}',
                  style: const TextStyle(
                    color: Color(0xFF666666),
                    fontSize: 16,
                    decoration: TextDecoration.lineThrough,
                    decorationColor: Color(0xFF666666),
                  ),
                ),
                const SizedBox(width: 8),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
                  decoration: BoxDecoration(
                    color: const Color(0x26FF2D55),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Text(
                    '-${product.discountPercentage!.toStringAsFixed(0)}% OFF',
                    style: const TextStyle(
                      color: Color(0xFFFF2D55),
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 10),
          // Rating + sold row
          Row(
            children: [
              ...List.generate(5, (i) {
                if (i < product.rating.floor()) {
                  return const Icon(Icons.star,
                      color: Color(0xFFFFC107), size: 16);
                } else if (i < product.rating) {
                  return const Icon(Icons.star_half,
                      color: Color(0xFFFFC107), size: 16);
                } else {
                  return const Icon(Icons.star_outline,
                      color: Color(0xFF444444), size: 16);
                }
              }),
              const SizedBox(width: 6),
              Text(
                '(${_formatCount(product.reviewCount)} reviews)',
                style: const TextStyle(
                    color: Color(0xFFAAAAAA), fontSize: 13),
              ),
              const SizedBox(width: 12),
              Text(
                '${_formatCount(product.soldCount)} sold',
                style: const TextStyle(
                    color: Color(0xFF666666), fontSize: 13),
              ),
            ],
          ),
          const SizedBox(height: 16),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Variants section
// ─────────────────────────────────────────────────────────────────────────────

class _VariantsSection extends StatelessWidget {
  const _VariantsSection({
    required this.product,
    required this.selectedVariantId,
    required this.onSelect,
  });

  final ProductEntity product;
  final String? selectedVariantId;
  final ValueChanged<String> onSelect;

  @override
  Widget build(BuildContext context) {
    // Group variants by their first attribute key (e.g. "Color", "Size")
    final attrKeys = product.variants
        .expand((v) => v.attributes.keys)
        .toSet()
        .toList();

    if (attrKeys.isEmpty) {
      // Simple list of named variants
      return Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Select variant:',
                style: TextStyle(
                    color: Color(0xFFAAAAAA),
                    fontSize: 13,
                    fontWeight: FontWeight.w500)),
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: product.variants.map((v) {
                final isSelected = v.id == selectedVariantId;
                return _SelectableChip(
                  label: v.name,
                  selected: isSelected,
                  available: v.isAvailable,
                  onTap: () => onSelect(v.id),
                );
              }).toList(),
            ),
          ],
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: attrKeys.map((key) {
          final values = product.variants
              .where((v) => v.attributes.containsKey(key))
              .map((v) => MapEntry(v.id, v.attributes[key]!))
              .toList();
          return Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('$key:',
                    style: const TextStyle(
                        color: Color(0xFFAAAAAA),
                        fontSize: 13,
                        fontWeight: FontWeight.w500)),
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: values.map((entry) {
                    final variant = product.variants
                        .firstWhere((v) => v.id == entry.key);
                    return _SelectableChip(
                      label: entry.value,
                      selected: selectedVariantId == entry.key,
                      available: variant.isAvailable,
                      onTap: () => onSelect(entry.key),
                    );
                  }).toList(),
                ),
              ],
            ),
          );
        }).toList(),
      ),
    );
  }
}

class _SelectableChip extends StatelessWidget {
  const _SelectableChip({
    required this.label,
    required this.selected,
    required this.available,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final bool available;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: available ? onTap : null,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 7),
        decoration: BoxDecoration(
          color: selected ? Colors.transparent : const Color(0xFF1A1A1A),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: selected
                ? const Color(0xFFFF2D55)
                : available
                    ? const Color(0xFF333333)
                    : const Color(0xFF222222),
            width: selected ? 1.5 : 1,
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: selected
                ? const Color(0xFFFF2D55)
                : available
                    ? Colors.white
                    : const Color(0xFF444444),
            fontSize: 13,
            fontWeight:
                selected ? FontWeight.w600 : FontWeight.w400,
            decoration:
                available ? null : TextDecoration.lineThrough,
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Seller row
// ─────────────────────────────────────────────────────────────────────────────

class _SellerRow extends StatelessWidget {
  const _SellerRow({required this.product});
  final ProductEntity product;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          CircleAvatar(
            radius: 20,
            backgroundColor: const Color(0xFF2A2A2A),
            backgroundImage: product.sellerAvatarUrl.isNotEmpty
                ? NetworkImage(product.sellerAvatarUrl)
                : null,
            child: product.sellerAvatarUrl.isEmpty
                ? Text(
                    product.sellerName.isNotEmpty
                        ? product.sellerName[0].toUpperCase()
                        : 'S',
                    style: const TextStyle(color: Colors.white, fontSize: 16),
                  )
                : null,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  product.sellerName,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const Text(
                  'Official Store',
                  style: TextStyle(color: Color(0xFF888888), fontSize: 12),
                ),
              ],
            ),
          ),
          OutlinedButton(
            onPressed: () {},
            style: OutlinedButton.styleFrom(
              foregroundColor: Colors.white,
              side: const BorderSide(color: Color(0xFF333333)),
              padding:
                  const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
              minimumSize: Size.zero,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8)),
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: const Text('Visit Store',
                style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500)),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Description
// ─────────────────────────────────────────────────────────────────────────────

class _DescriptionSection extends StatelessWidget {
  const _DescriptionSection({
    required this.description,
    required this.expanded,
    required this.onToggle,
  });

  final String description;
  final bool expanded;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Description',
            style: TextStyle(
              color: Colors.white,
              fontSize: 15,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 8),
          AnimatedCrossFade(
            duration: const Duration(milliseconds: 250),
            crossFadeState: expanded
                ? CrossFadeState.showSecond
                : CrossFadeState.showFirst,
            firstChild: Text(
              description,
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                  color: Color(0xFFAAAAAA),
                  fontSize: 14,
                  height: 1.5),
            ),
            secondChild: Text(
              description,
              style: const TextStyle(
                  color: Color(0xFFAAAAAA),
                  fontSize: 14,
                  height: 1.5),
            ),
          ),
          if (description.length > 120)
            GestureDetector(
              onTap: onToggle,
              child: Padding(
                padding: const EdgeInsets.only(top: 6),
                child: Text(
                  expanded ? 'Show less' : 'Read more',
                  style: const TextStyle(
                    color: Color(0xFFFF2D55),
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Reviews section
// ─────────────────────────────────────────────────────────────────────────────

class _ReviewsSection extends StatelessWidget {
  const _ReviewsSection({required this.product});
  final ProductEntity product;

  // Synthetic distribution based on average rating
  List<double> get _distribution {
    final avg = product.rating;
    return [
      (avg >= 4.5 ? 0.6 : avg >= 4.0 ? 0.45 : 0.3),
      (avg >= 4.0 ? 0.25 : 0.25),
      0.08,
      0.04,
      0.03,
    ];
  }

  @override
  Widget build(BuildContext context) {
    final dist = _distribution;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Reviews',
            style: TextStyle(
              color: Colors.white,
              fontSize: 15,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Big rating number
              Column(
                children: [
                  Text(
                    product.rating.toStringAsFixed(1),
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 48,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                  Row(
                    children: List.generate(
                      5,
                      (i) => Icon(
                        i < product.rating.round()
                            ? Icons.star
                            : Icons.star_outline,
                        color: const Color(0xFFFFC107),
                        size: 14,
                      ),
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${_formatCount(product.reviewCount)} reviews',
                    style: const TextStyle(
                        color: Color(0xFF666666), fontSize: 11),
                  ),
                ],
              ),
              const SizedBox(width: 20),
              // Bar chart
              Expanded(
                child: Column(
                  children: List.generate(5, (i) {
                    final star = 5 - i;
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 3),
                      child: Row(
                        children: [
                          Text(
                            '$star',
                            style: const TextStyle(
                                color: Color(0xFF888888), fontSize: 12),
                          ),
                          const SizedBox(width: 4),
                          const Icon(Icons.star,
                              color: Color(0xFFFFC107), size: 10),
                          const SizedBox(width: 6),
                          Expanded(
                            child: ClipRRect(
                              borderRadius: BorderRadius.circular(4),
                              child: LinearProgressIndicator(
                                value: dist[i],
                                backgroundColor: const Color(0xFF2A2A2A),
                                valueColor:
                                    const AlwaysStoppedAnimation<Color>(
                                        Color(0xFFFFC107)),
                                minHeight: 6,
                              ),
                            ),
                          ),
                          const SizedBox(width: 6),
                          Text(
                            '${(dist[i] * 100).toStringAsFixed(0)}%',
                            style: const TextStyle(
                                color: Color(0xFF666666), fontSize: 11),
                          ),
                        ],
                      ),
                    );
                  }),
                ),
              ),
            ],
          ),
          const SizedBox(height: 20),
          // Sample reviews
          ..._sampleReviews(product).map(
            (review) => Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: _ReviewTile(review: review),
            ),
          ),
        ],
      ),
    );
  }

  List<_Review> _sampleReviews(ProductEntity product) {
    return [
      _Review(
        name: 'Alex M.',
        rating: 5,
        comment:
            'Absolutely love this! Great quality and fast shipping. Exactly as described.',
        date: '2 days ago',
      ),
      _Review(
        name: 'Jamie T.',
        rating: 4,
        comment:
            'Really good product. Fits well and looks even better in person.',
        date: '1 week ago',
      ),
      _Review(
        name: 'Sam K.',
        rating: (product.rating).clamp(3, 5).toInt() == 5 ? 5 : 4,
        comment:
            'Good value for the price. Would recommend to friends.',
        date: '2 weeks ago',
      ),
    ];
  }
}

class _Review {
  const _Review({
    required this.name,
    required this.rating,
    required this.comment,
    required this.date,
  });
  final String name;
  final int rating;
  final String comment;
  final String date;
}

class _ReviewTile extends StatelessWidget {
  const _ReviewTile({required this.review});
  final _Review review;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            CircleAvatar(
              radius: 16,
              backgroundColor: const Color(0xFF2A2A2A),
              child: Text(
                review.name[0],
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.w600),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(review.name,
                      style: const TextStyle(
                          color: Colors.white,
                          fontSize: 13,
                          fontWeight: FontWeight.w500)),
                  Row(
                    children: [
                      ...List.generate(
                          5,
                          (i) => Icon(
                                i < review.rating
                                    ? Icons.star
                                    : Icons.star_outline,
                                color: const Color(0xFFFFC107),
                                size: 12,
                              )),
                      const SizedBox(width: 6),
                      Text(review.date,
                          style: const TextStyle(
                              color: Color(0xFF666666), fontSize: 11)),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          review.comment,
          style: const TextStyle(
              color: Color(0xFFAAAAAA), fontSize: 13, height: 1.5),
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Sticky bottom bar
// ─────────────────────────────────────────────────────────────────────────────

class _StickyBottomBar extends StatelessWidget {
  const _StickyBottomBar({
    required this.product,
    required this.qty,
    required this.onQtyChange,
    required this.selectedVariantId,
    required this.addingToCart,
    required this.onAddToCart,
    required this.onBuyNow,
  });

  final ProductEntity product;
  final int qty;
  final ValueChanged<int> onQtyChange;
  final String? selectedVariantId;
  final bool addingToCart;
  final VoidCallback onAddToCart;
  final VoidCallback onBuyNow;

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    return Container(
      padding: EdgeInsets.fromLTRB(16, 12, 16, 12 + bottomPadding),
      decoration: const BoxDecoration(
        color: Color(0xFF0A0A0A),
        border: Border(top: BorderSide(color: Color(0xFF1A1A1A))),
      ),
      child: Row(
        children: [
          // Qty stepper
          Container(
            decoration: BoxDecoration(
              color: const Color(0xFF1A1A1A),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _QtyButton(
                  icon: Icons.remove,
                  onTap: qty > 1 ? () => onQtyChange(qty - 1) : null,
                ),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  child: Text(
                    '$qty',
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                _QtyButton(
                  icon: Icons.add,
                  onTap: () => onQtyChange(qty + 1),
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          // Add to Cart
          Expanded(
            child: OutlinedButton(
              onPressed: product.inStock && !addingToCart ? onAddToCart : null,
              style: OutlinedButton.styleFrom(
                foregroundColor: const Color(0xFFFF2D55),
                side: const BorderSide(color: Color(0xFFFF2D55)),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
              child: addingToCart
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Color(0xFFFF2D55)),
                    )
                  : const Text('Add to Cart',
                      style: TextStyle(
                          fontSize: 13, fontWeight: FontWeight.w600)),
            ),
          ),
          const SizedBox(width: 10),
          // Buy Now
          Expanded(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: const LinearGradient(
                  colors: [Color(0xFFFF2D55), Color(0xFFFF6B35)],
                ),
                borderRadius: BorderRadius.circular(10),
              ),
              child: ElevatedButton(
                onPressed: product.inStock ? onBuyNow : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.transparent,
                  shadowColor: Colors.transparent,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10)),
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                child: const Text(
                  'Buy Now',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _QtyButton extends StatelessWidget {
  const _QtyButton({required this.icon, required this.onTap});
  final IconData icon;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: Icon(
          icon,
          size: 18,
          color: onTap != null ? Colors.white : const Color(0xFF444444),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────────────────────────────────────

class _Divider extends StatelessWidget {
  const _Divider();

  @override
  Widget build(BuildContext context) {
    return const Divider(
        color: Color(0xFF1A1A1A), thickness: 1, height: 1);
  }
}

String _formatCount(int count) {
  if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
  if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
  return count.toString();
}
