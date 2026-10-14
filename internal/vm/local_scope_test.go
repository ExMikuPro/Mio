package vm

import (
	"bytes"
	"testing"
)

// 测试代码块变量的遮蔽、嵌套和外层赋值
func TestVMBlockVariableScopes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		globalSlots int
	}{
		{
			name: "局部变量遮蔽全局变量",
			input: `
kaku value = 10;
moshi (hai) {
    kaku value = 20;
    chant(value);
}
chant(value);
`,
			want:        "20\n10\n",
			globalSlots: 2,
		},
		{
			name: "遮蔽声明初始化读取外层变量",
			input: `
kaku value = 10;
moshi (hai) {
    kaku value = value + 1;
    chant(value);
}
chant(value);
`,
			want:        "11\n10\n",
			globalSlots: 2,
		},
		{
			name: "内层代码块修改外层变量",
			input: `
kaku count = 1;
moshi (hai) {
    count = count + 1;
    kaku local = 100;
    chant(local);
}
chant(count);
`,
			want:        "100\n2\n",
			globalSlots: 2,
		},
		{
			name: "三层嵌套作用域",
			input: `
kaku value = 1;
moshi (hai) {
    kaku value = 2;
    moshi (hai) {
        kaku local = 3;
        value = value + 2;
        chant(local);
        chant(value);
    }
    chant(value);
}
chant(value);
`,
			want:        "3\n4\n4\n1\n",
			globalSlots: 3,
		},
		{
			name: "相邻代码块使用相同变量名",
			input: `
moshi (hai) {
    kaku local = 1;
    chant(local);
}
moshi (hai) {
    kaku local = 2;
    chant(local);
}
`,
			want:        "1\n2\n",
			globalSlots: 2,
		},
		{
			name: "moshi 分支声明变量",
			input: `
moshi (hai) {
    kaku local = 10;
    chant(local);
} sore {
    kaku local = 20;
    chant(local);
}
`,
			want:        "10\n",
			globalSlots: 2,
		},
		{
			name: "sore 分支声明变量",
			input: `
moshi (iie) {
    kaku local = 10;
    chant(local);
} sore {
    kaku local = 20;
    chant(local);
}
`,
			want:        "20\n",
			globalSlots: 2,
		},
		{
			name: "条件为假时不执行初始化",
			input: `
kaku count = 0;
moshi (iie) {
    kaku local = 1 / 0;
    chant(local);
}
chant(count);
`,
			want:        "0\n",
			globalSlots: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytecode := compileVariableSource(t, tt.input)
			if bytecode.GlobalsCount != tt.globalSlots {
				t.Fatalf("global count: got %d, want %d", bytecode.GlobalsCount, tt.globalSlots)
			}

			var output bytes.Buffer
			machine := NewWithOutput(bytecode, &output)
			if err := machine.Run(); err != nil {
				t.Fatal(err)
			}
			if output.String() != tt.want {
				t.Fatalf("output: got %q, want %q", output.String(), tt.want)
			}
			if len(machine.stack) != 0 {
				t.Fatalf("expected empty stack, got %v", machine.stack)
			}
		})
	}
}
