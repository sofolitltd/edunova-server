package omr

import "errors"

// Homography maps points from template mm-space into photographed
// pixel-space via a projective transform, computed from 4 point
// correspondences (the 4 fiducial markers).
type Homography struct {
	m [3][3]float64
}

// ComputeHomography solves for the projective transform mapping src[i] -> dst[i]
// for the 4 corner correspondences, using the standard Direct Linear
// Transform (DLT) reduced to an 8x8 linear system (the 9th homogeneous
// coordinate is fixed to 1).
func ComputeHomography(src, dst [4]Point) (Homography, error) {
	// Build the 8x8 system A*h = b for h = [a,b,c,d,e,f,g,h] where
	// the homography matrix is [[a,b,c],[d,e,f],[g,h,1]].
	var a [8][8]float64
	var b [8]float64

	for i := 0; i < 4; i++ {
		x, y := src[i].XMM, src[i].YMM
		u, v := dst[i].XMM, dst[i].YMM

		row := i * 2
		a[row] = [8]float64{x, y, 1, 0, 0, 0, -x * u, -y * u}
		b[row] = u

		row2 := i*2 + 1
		a[row2] = [8]float64{0, 0, 0, x, y, 1, -x * v, -y * v}
		b[row2] = v
	}

	h, err := solveLinearSystem(a, b)
	if err != nil {
		return Homography{}, err
	}

	var hg Homography
	hg.m = [3][3]float64{
		{h[0], h[1], h[2]},
		{h[3], h[4], h[5]},
		{h[6], h[7], 1},
	}
	return hg, nil
}

// Apply maps a template-space point through the homography into pixel space.
func (hg Homography) Apply(p Point) (float64, float64) {
	x, y := p.XMM, p.YMM
	w := hg.m[2][0]*x + hg.m[2][1]*y + hg.m[2][2]
	if w == 0 {
		w = 1e-9
	}
	px := (hg.m[0][0]*x + hg.m[0][1]*y + hg.m[0][2]) / w
	py := (hg.m[1][0]*x + hg.m[1][1]*y + hg.m[1][2]) / w
	return px, py
}

// solveLinearSystem solves the 8x8 system a*x = b via Gaussian elimination
// with partial pivoting.
func solveLinearSystem(a [8][8]float64, b [8]float64) ([8]float64, error) {
	const n = 8
	// Augmented matrix.
	var mat [n][n + 1]float64
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			mat[i][j] = a[i][j]
		}
		mat[i][n] = b[i]
	}

	for col := 0; col < n; col++ {
		pivot := col
		maxAbs := abs(mat[col][col])
		for row := col + 1; row < n; row++ {
			if v := abs(mat[row][col]); v > maxAbs {
				pivot = row
				maxAbs = v
			}
		}
		if maxAbs < 1e-9 {
			return [8]float64{}, errors.New("omr: singular homography system (markers not detected correctly)")
		}
		mat[col], mat[pivot] = mat[pivot], mat[col]

		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := mat[row][col] / mat[col][col]
			for k := col; k <= n; k++ {
				mat[row][k] -= factor * mat[col][k]
			}
		}
	}

	var x [8]float64
	for i := 0; i < n; i++ {
		x[i] = mat[i][n] / mat[i][i]
	}
	return x, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
