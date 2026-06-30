import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../ecommerce/domain/entities/order_entity.dart';
import '../../../ecommerce/presentation/providers/ecommerce_provider.dart';

// ─────────────────────────────────────────────────────────────────────────────
// CartScreen
// ─────────────────────────────────────────────────────────────────────────────

class CartScreen extends ConsumerWidget {
  const CartScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncCart = ref.watch(cartProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back, color: Colors.white),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: asyncCart.when(
          data: (cart) => Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text(
                'Cart',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.w700),
              ),
              if (cart.itemCount > 0) ...[
                const SizedBox(width: 8),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                  decoration: BoxDecoration(
                    color: const Color(0xFFFF2D55),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    '${cart.itemCount}',
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ],
            ],
          ),
          loading: () => const Text('Cart',
              style: TextStyle(color: Colors.white, fontSize: 18)),
          error: (_, __) => const Text('Cart',
              style: TextStyle(color: Colors.white, fontSize: 18)),
        ),
      ),
      body: asyncCart.when(
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
                onPressed: () => ref.invalidate(cartProvider),
                child: const Text('Retry',
                    style: TextStyle(color: Color(0xFFFF2D55))),
              ),
            ],
          ),
        ),
        data: (cart) {
          if (cart.isEmpty) return const _EmptyCart();
          return Column(
            children: [
              Expanded(
                child: ListView.separated(
                  padding: const EdgeInsets.symmetric(
                      horizontal: 16, vertical: 12),
                  itemCount: cart.items.length,
                  separatorBuilder: (_, __) => const SizedBox(height: 12),
                  itemBuilder: (context, index) {
                    return _CartItemTile(item: cart.items[index]);
                  },
                ),
              ),
              _OrderSummaryCard(cart: cart),
            ],
          );
        },
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Empty state
// ─────────────────────────────────────────────────────────────────────────────

class _EmptyCart extends StatelessWidget {
  const _EmptyCart();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.shopping_bag_outlined,
              color: Color(0xFF333333), size: 80),
          const SizedBox(height: 20),
          const Text(
            'Your cart is empty',
            style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          const Text(
            'Add items from the shop to get started.',
            style: TextStyle(color: Color(0xFF888888), fontSize: 14),
          ),
          const SizedBox(height: 28),
          ElevatedButton(
            onPressed: () => Navigator.of(context).pop(),
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFFFF2D55),
              foregroundColor: Colors.white,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
              padding:
                  const EdgeInsets.symmetric(horizontal: 32, vertical: 14),
            ),
            child: const Text('Browse Shop',
                style: TextStyle(fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Cart item tile
// ─────────────────────────────────────────────────────────────────────────────

class _CartItemTile extends ConsumerStatefulWidget {
  const _CartItemTile({required this.item});
  final CartItemEntity item;

  @override
  ConsumerState<_CartItemTile> createState() => _CartItemTileState();
}

class _CartItemTileState extends ConsumerState<_CartItemTile> {
  bool _removing = false;

  Future<void> _remove() async {
    setState(() => _removing = true);
    await ref.read(cartProvider.notifier).removeItem(widget.item.id);
    // Widget may be unmounted after removal; no setState needed.
  }

  @override
  Widget build(BuildContext context) {
    final item = widget.item;
    final variant = item.selectedVariant;

    return AnimatedOpacity(
      opacity: _removing ? 0.4 : 1.0,
      duration: const Duration(milliseconds: 200),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: const Color(0xFF111111),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Thumbnail
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: SizedBox(
                width: 80,
                height: 80,
                child: item.product.thumbnailUrl.isNotEmpty
                    ? Image.network(
                        item.product.thumbnailUrl,
                        fit: BoxFit.cover,
                        errorBuilder: (_, __, ___) => Container(
                          color: const Color(0xFF2A2A2A),
                          child: const Icon(Icons.broken_image_outlined,
                              color: Color(0xFF444444)),
                        ),
                        loadingBuilder: (_, child, progress) {
                          if (progress == null) return child;
                          return Container(color: const Color(0xFF2A2A2A));
                        },
                      )
                    : Container(
                        color: const Color(0xFF2A2A2A),
                        child: const Icon(Icons.image_not_supported_outlined,
                            color: Color(0xFF444444)),
                      ),
              ),
            ),
            const SizedBox(width: 12),
            // Info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.product.name,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                  if (variant != null) ...[
                    const SizedBox(height: 4),
                    Text(
                      variant.name,
                      style: const TextStyle(
                          color: Color(0xFF888888), fontSize: 12),
                    ),
                  ],
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Text(
                        '\$${item.unitPrice.toStringAsFixed(2)}',
                        style: const TextStyle(
                          color: Color(0xFFFF2D55),
                          fontSize: 15,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const Spacer(),
                      // Qty stepper
                      _InlineQtyStepper(item: item),
                    ],
                  ),
                ],
              ),
            ),
            // Delete
            GestureDetector(
              onTap: _removing ? null : _remove,
              child: Padding(
                padding: const EdgeInsets.only(left: 8),
                child: _removing
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(
                            strokeWidth: 1.5,
                            color: Color(0xFFFF2D55)),
                      )
                    : const Icon(Icons.delete_outline,
                        color: Color(0xFF666666), size: 20),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _InlineQtyStepper extends ConsumerWidget {
  const _InlineQtyStepper({required this.item});
  final CartItemEntity item;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      decoration: BoxDecoration(
        color: const Color(0xFF2A2A2A),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _StepBtn(
            icon: Icons.remove,
            onTap: () => ref
                .read(cartProvider.notifier)
                .updateQty(itemId: item.id, qty: item.qty - 1),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 10),
            child: Text(
              '${item.qty}',
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w600),
            ),
          ),
          _StepBtn(
            icon: Icons.add,
            onTap: () => ref
                .read(cartProvider.notifier)
                .updateQty(itemId: item.id, qty: item.qty + 1),
          ),
        ],
      ),
    );
  }
}

class _StepBtn extends StatelessWidget {
  const _StepBtn({required this.icon, required this.onTap});
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
        child: Icon(icon, size: 16, color: Colors.white),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Order summary card
// ─────────────────────────────────────────────────────────────────────────────

class _OrderSummaryCard extends StatelessWidget {
  const _OrderSummaryCard({required this.cart});
  final CartEntity cart;

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    return Container(
      padding: EdgeInsets.fromLTRB(16, 16, 16, 16 + bottomPadding),
      decoration: const BoxDecoration(
        color: Color(0xFF111111),
        border: Border(top: BorderSide(color: Color(0xFF1A1A1A))),
      ),
      child: Column(
        children: [
          _SummaryRow(
              label: 'Subtotal',
              value: '\$${cart.subtotal.toStringAsFixed(2)}'),
          const SizedBox(height: 6),
          _SummaryRow(
            label: 'Shipping',
            value: cart.shippingFee == 0
                ? 'Free'
                : '\$${cart.shippingFee.toStringAsFixed(2)}',
          ),
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 10),
            child:
                Divider(color: Color(0xFF2A2A2A), thickness: 1, height: 1),
          ),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Total',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w700),
              ),
              Text(
                '\$${cart.total.toStringAsFixed(2)}',
                style: const TextStyle(
                    color: Color(0xFFFF2D55),
                    fontSize: 20,
                    fontWeight: FontWeight.w800),
              ),
            ],
          ),
          const SizedBox(height: 14),
          SizedBox(
            width: double.infinity,
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: const LinearGradient(
                  colors: [Color(0xFFFF2D55), Color(0xFFFF6B35)],
                ),
                borderRadius: BorderRadius.circular(12),
              ),
              child: ElevatedButton(
                onPressed: () =>
                    Navigator.of(context).pushNamed('/checkout'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.transparent,
                  shadowColor: Colors.transparent,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                ),
                child: const Text(
                  'Checkout',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w700),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SummaryRow extends StatelessWidget {
  const _SummaryRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label,
            style: const TextStyle(
                color: Color(0xFF888888), fontSize: 14)),
        Text(value,
            style: const TextStyle(color: Colors.white, fontSize: 14)),
      ],
    );
  }
}
